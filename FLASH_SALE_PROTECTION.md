# Flash Sale Attack Protection - Technical Explanation

## Attack Scenario
**50 concurrent requests for a coupon with only 5 items in stock**
- Expected Result: **Exactly 5 claims succeed, 45 fail, 0 remaining stock**

## Defense Mechanisms

The protection against overselling in a flash sale happens through **atomic stock management** within MongoDB transactions.

---

## 1. Atomic Stock Decrement (PRIMARY DEFENSE)

**Location**: MongoDB repository implementation (`ClaimCoupon` method)

**What it does**: Uses MongoDB's atomic `$inc` operator with a condition to ensure stock can only be decremented if it's greater than 0.

**Key Implementation** (pseudo-code for MongoDB repository):

```go
func (r *mongoCouponRepository) ClaimCoupon(ctx context.Context, userID, couponName string) error {
    session, err := r.client.StartSession()
    if err != nil {
        return err
    }
    defer session.EndSession(ctx)

    _, err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
        // 1. Get coupon by name to retrieve its ID
        var coupon model.Coupon
        err := r.couponsCollection.FindOne(sc, bson.M{"name": couponName}).Decode(&coupon)
        if err != nil {
            return ErrCouponNotFound
        }

        // 2. Check if user already claimed (using coupon_id)
        existingClaim := r.claimsCollection.FindOne(sc, bson.M{
            "user_id":   userID,
            "coupon_id": coupon.ID,
        })
        if existingClaim.Err() == nil {
            return ErrAlreadyClaimed
        }

        // 3. ATOMIC STOCK DECREMENT - This is the critical part!
        // Uses FindOneAndUpdate with condition to atomically:
        //   - Check stock > 0
        //   - Decrement by 1
        //   - Return updated document
        updateResult := r.couponsCollection.FindOneAndUpdate(
            sc,
            bson.M{
                "_id":             coupon.ID,
                "remaining_amount": bson.M{"$gt": 0}, // CRITICAL: Only update if stock > 0
            },
            bson.M{"$inc": bson.M{"remaining_amount": -1}}, // Atomic decrement
            options.FindOneAndUpdate().
                SetReturnDocument(options.After). // Return updated document
                SetUpsert(false),
        )
        
        if updateResult.Err() != nil {
            // This means either:
            // - Coupon not found (shouldn't happen after step 1)
            // - Stock was already 0 or went negative
            return ErrNoStock
        }

        // 4. Insert claim record (only if stock decrement succeeded)
        claim := &model.Claim{
            UserID:     userID,
            CouponID:   coupon.ID,
            CouponName: couponName,
            CreatedAt:  time.Now(),
        }
        _, err = r.claimsCollection.InsertOne(sc, claim)
        if err != nil {
            // If this fails, transaction will rollback (stock will be restored)
            if mongo.IsDuplicateKeyError(err) {
                return ErrAlreadyClaimed
            }
            return err
        }

        return nil // Transaction commits
    })

    return err
}
```

**Why it works**:
- **Atomic Operation**: `FindOneAndUpdate` with `$inc` is atomic at the document level
- **Conditional Update**: The condition `"remaining_amount": {"$gt": 0}` ensures the update only happens if stock > 0
- **Single Operation**: Stock check and decrement happen in ONE atomic operation, preventing race conditions
- **Transaction Safety**: If claim insert fails, the entire transaction rolls back (stock restored)

---

## 2. Transaction Isolation

**What it does**: All operations (stock check, decrement, claim insert) happen within a single MongoDB transaction.

**Benefits**:
- **ACID Guarantees**: Either all operations succeed or all fail
- **Isolation**: Other transactions see consistent state
- **Rollback**: If any step fails, stock is not decremented

---

## 3. Race Condition Handling

### Scenario: 50 Concurrent Requests, 5 Items in Stock

```
Time    Request 1-5        Request 6-50
T0      Start Transaction  Start Transaction
T1      Get coupon         Get coupon
        (stock = 5)        (stock = 5)
T2      FindOneAndUpdate   FindOneAndUpdate
        (condition:        (condition:
         stock > 0)         stock > 0)
        ↓                   ↓
        ┌───────────────────┐
        │ MongoDB processes │
        │ requests in order │
        └───────────────────┘
        ↓                   ↓
T3      ✓ SUCCESS          ✗ FAIL
        (stock: 5→4)       (condition false:
        Insert claim        stock now ≤ 0)
        ✓ SUCCESS          Transaction aborted
                           Return ErrNoStock
```

**What happens**:
1. **First 5 requests**: Each `FindOneAndUpdate` succeeds because stock > 0
   - Request 1: stock 5 → 4 ✓
   - Request 2: stock 4 → 3 ✓
   - Request 3: stock 3 → 2 ✓
   - Request 4: stock 2 → 1 ✓
   - Request 5: stock 1 → 0 ✓

2. **Remaining 45 requests**: `FindOneAndUpdate` fails because condition `remaining_amount > 0` is now false
   - All return `ErrNoStock`
   - No stock decremented
   - Transaction aborted

3. **Result**: Exactly 5 claims, 0 remaining stock

---

## 4. Why FindOneAndUpdate is Critical

**Traditional (WRONG) Approach**:
```go
// ❌ WRONG - Race condition!
coupon := getCoupon()           // stock = 5
if coupon.RemainingAmount > 0 { // 50 requests all see stock = 5
    coupon.RemainingAmount--     // All 50 decrement!
    updateCoupon(coupon)         // Result: -45 stock (oversold!)
    insertClaim()
}
```

**Correct Approach**:
```go
// ✅ CORRECT - Atomic operation
updateResult := FindOneAndUpdate(
    {"_id": couponID, "remaining_amount": {"$gt": 0}}, // Condition
    {"$inc": {"remaining_amount": -1}}                   // Atomic decrement
)
// Only succeeds if condition is true at the moment of update
// MongoDB guarantees atomicity - only 5 can succeed
```

**Key Difference**:
- **Traditional**: Check and update are separate → race condition
- **Atomic**: Check and update are ONE operation → no race condition

---

## 5. Complete Flow Diagram

```
50 Concurrent Requests
         ↓
    [Handler Layer]
         ↓
    [Service Layer] ← Line 53-55: Delegates to repository
         ↓
    [Repository Layer] ← MongoDB Transaction
         ↓
    ┌─────────────────────────────────────────┐
    │ MongoDB Transaction (per request)        │
    │                                          │
    │ 1. Get coupon by name                   │
    │    (all see stock = 5)                   │
    │                                          │
    │ 2. Check existing claim                 │
    │    (if user already claimed)             │
    │                                          │
    │ 3. ATOMIC STOCK DECREMENT                │
    │    FindOneAndUpdate(                     │
    │      condition: stock > 0,              │
    │      operation: $inc -1                  │
    │    )                                     │
    │    ↓                                     │
    │    ┌─────────────────────┐              │
    │    │ MongoDB processes    │              │
    │    │ atomically in order  │              │
    │    └─────────────────────┘              │
    │    ↓                                     │
    │    Request 1-5: ✓ SUCCESS               │
    │    (stock: 5→4→3→2→1→0)                  │
    │    Request 6-50: ✗ FAIL                 │
    │    (condition false: stock = 0)          │
    │                                          │
    │ 4. Insert claim (only if step 3 ✓)      │
    │                                          │
    │ 5. Commit transaction                    │
    └─────────────────────────────────────────┘
         ↓
    Result:
    - 5 successful claims
    - 45 failures (ErrNoStock)
    - 0 remaining stock
```

---

## 6. Code Locations

### Current Implementation

**`internal/service/coupon_service.go:53-55`**
```go
func (s *CouponService) ClaimCoupon(ctx context.Context, req *model.ClaimCouponRequest) error {
    return s.repo.ClaimCoupon(ctx, req.UserID, req.CouponName)
}
```
- Delegates to repository (stock management happens at repository level)

**`internal/repository/coupon_repository.go:16-21`**
```go
// ClaimCoupon attempts to claim a coupon atomically
// Takes couponName, looks up coupon to get its ID, then uses coupon_id for the claim
// Returns:
//   - nil: success
//   - error: if claim failed (already claimed, no stock, etc.)
ClaimCoupon(ctx context.Context, userID, couponName string) error
```
- Interface contract: must be atomic and handle stock exhaustion

### To Be Implemented

**MongoDB Repository Implementation** must:
1. Use MongoDB session with transaction support
2. Use `FindOneAndUpdate` with condition `remaining_amount > 0`
3. Use atomic `$inc` operator for stock decrement
4. Only insert claim if stock decrement succeeded
5. Return `ErrNoStock` when `FindOneAndUpdate` fails

---

## 7. Edge Cases Handled

### Case 1: Stock Exhausted Mid-Transaction
- **Scenario**: Request checks stock (5 available), but by the time it tries to decrement, stock is 0
- **Solution**: `FindOneAndUpdate` condition fails, returns `ErrNoStock`, transaction aborts

### Case 2: Multiple Users, Same Coupon
- **Scenario**: 50 different users trying to claim the same coupon
- **Solution**: First 5 succeed (unique user_id + coupon_id combinations), rest fail with `ErrNoStock`

### Case 3: Same User, Different Coupons
- **Scenario**: User tries to claim multiple different coupons
- **Solution**: Each coupon has separate stock, unique index only prevents same user claiming same coupon twice

### Case 4: Transaction Failure After Stock Decrement
- **Scenario**: Stock decremented successfully, but claim insert fails
- **Solution**: Transaction rolls back, stock is restored (ACID guarantee)

---

## 8. Performance Considerations

**Why This Approach is Efficient**:
1. **Single Database Operation**: Stock check + decrement in one atomic operation
2. **No Application-Level Locking**: MongoDB handles concurrency at database level
3. **Minimal Lock Duration**: Transaction only holds locks during the operation
4. **Index Usage**: Unique index on `(user_id, coupon_id)` is efficient for duplicate checks

**Scalability**:
- Can handle thousands of concurrent requests
- MongoDB's write concern ensures durability
- Transaction overhead is minimal for this use case

---

## Summary: Key Takeaways

1. **Atomic Stock Decrement**: `FindOneAndUpdate` with `$inc` and condition ensures only valid decrements succeed
2. **Transaction Safety**: All operations wrapped in transaction for ACID guarantees
3. **Conditional Update**: `remaining_amount > 0` condition prevents overselling
4. **Single Operation**: Stock check and decrement are atomic - no race conditions possible
5. **Automatic Rollback**: Failed transactions restore stock automatically

**The combination of atomic operations and transactions guarantees that exactly 5 claims succeed and 45 fail, with 0 remaining stock, even under extreme concurrency.**

