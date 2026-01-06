# Scalable Coupon System API

A high-performance REST API for a "Flash Sale" Coupon System built with Golang, MongoDB, and Docker. The system handles high concurrency, guarantees strict data consistency, and prevents both overselling and duplicate claims.

## Features

- ✅ **High Concurrency**: Handles 50+ concurrent requests with atomic operations
- ✅ **Data Consistency**: MongoDB transactions ensure ACID guarantees
- ✅ **Double-Dip Protection**: Unique compound index prevents users from claiming the same coupon twice
- ✅ **Flash Sale Protection**: Atomic stock decrement prevents overselling
- ✅ **Docker Deployment**: One-command deployment with Docker Compose

## Prerequisites

- **Docker Desktop** (or Docker Engine + Docker Compose)
- **Go 1.21+** (for local development)
- **MongoDB Database Tools** (for seeding data, optional)

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone and navigate to the project**:
   ```bash
   cd /path/to/project
   ```

2. **Start the application**:
   ```bash
   docker-compose up --build
   ```

3. **Seed the database** (in a new terminal):
   ```bash
   # Wait for MongoDB to be ready, then run:
   ./seed/seed_mongodb.sh
   ```

4. **Test the API**:
   ```bash
   # Health check
   curl http://localhost:8080/health

   # Create a coupon
   curl -X POST http://localhost:8080/api/coupons \
     -H "Content-Type: application/json" \
     -d '{"name": "PROMO_TEST", "amount": 1000}'

   # Claim a coupon
   curl -X POST http://localhost:8080/api/coupons/claim \
     -H "Content-Type: application/json" \
     -d '{"user_id": "user_123", "coupon_name": "FLASH_SALE_2024"}'

   # Get coupon details
   curl http://localhost:8080/api/coupons/FLASH_SALE_2024
   ```

### Local Development

1. **Start MongoDB** (using Docker):
   ```bash
   docker run -d -p 27017:27017 --name mongodb mongo:7.0
   ```

2. **Seed the database**:
   ```bash
   ./seed/seed_mongodb.sh
   ```

3. **Run the application**:
   ```bash
   go run cmd/server/main.go
   ```

## API Endpoints

### 1. Create Coupon

**Endpoint**: `POST /api/coupons`

**Request Body**:
```json
{
  "name": "PROMO_SUPER",
  "amount": 100
}
```

**Response**: `201 Created`
```json
{
  "id": "...",
  "name": "PROMO_SUPER",
  "amount": 100,
  "remaining_amount": 100,
  ...
}
```

### 2. Claim Coupon

**Endpoint**: `POST /api/coupons/claim`

**Request Body**:
```json
{
  "user_id": "user_12345",
  "coupon_name": "PROMO_SUPER"
}
```

**Response Codes**:
- `200 OK` - Success
- `409 Conflict` - Already claimed by this user
- `400 Bad Request` - No stock available
- `404 Not Found` - Coupon not found

### 3. Get Coupon Details

**Endpoint**: `GET /api/coupons/{name}`

**Response**: `200 OK`
```json
{
  "name": "PROMO_SUPER",
  "amount": 100,
  "remaining_amount": 0,
  "claimed_by": ["user_12345", "user_67890"]
}
```

**Note**: All amounts are in **cents**.

## Testing

### Unit Tests

```bash
go test ./internal/service/...
```

### Integration Testing

The seed data includes a Flash Sale coupon with 5 items in stock. You can test concurrent requests:

```bash
# Test Flash Sale scenario (50 concurrent requests)
for i in {1..50}; do
  curl -X POST http://localhost:8080/api/coupons/claim \
    -H "Content-Type: application/json" \
    -d "{\"user_id\":\"user_$i\",\"coupon_name\":\"FLASH_SALE_2024\"}" &
done
wait

# Check results
curl http://localhost:8080/api/coupons/FLASH_SALE_2024
```

**Expected Result**: Exactly 5 successful claims, 45 failures, 0 remaining stock.

### Test Scenarios

1. **Flash Sale Attack**: 50 concurrent requests for a coupon with 5 items
   - Expected: 5 successes, 45 failures, 0 remaining

2. **Double Dip Attack**: 10 concurrent requests from the same user for the same coupon
   - Expected: 1 success, 9 failures (409 Conflict)

## Architecture

### Database Design

**Collections**:
- `coupons`: Stores coupon information
- `claims`: Stores claim history (separate collection, no embedding)

**Indexes**:
- `coupons.name`: Unique index
- `claims(user_id, coupon_id)`: Unique compound index (prevents double-dip)
- `claims.coupon_id`: Index for faster lookups
- `claims.coupon_name`: Index for querying

### Concurrency Strategy

1. **MongoDB Multi-Document Transactions**: All claim operations are atomic
2. **Atomic Stock Decrement**: Uses `FindOneAndUpdate` with `$inc` operator and condition
3. **Unique Compound Index**: Database-level constraint prevents duplicate claims
4. **Transaction Isolation**: ACID guarantees across multiple collections

### Key Implementation Details

#### Flash Sale Protection
- Uses `FindOneAndUpdate` with condition `remaining_amount > 0`
- Atomic `$inc` operator ensures only valid decrements succeed
- Even with 50 concurrent requests, exactly 5 succeed

#### Double Dip Protection
- Unique compound index on `(user_id, coupon_id)`
- MongoDB rejects duplicate inserts automatically
- Returns 409 Conflict for duplicate claims

## Project Structure

```
/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── handler/
│   │   └── coupon_handler.go   # HTTP handlers
│   ├── service/
│   │   ├── coupon_service.go   # Business logic
│   │   └── coupon_service_test.go # Unit tests
│   ├── repository/
│   │   ├── coupon_repository.go # Repository interface
│   │   └── mongo_repository.go  # MongoDB implementation
│   └── model/
│       └── coupon.go            # Data models
├── pkg/
│   └── database/
│       └── mongodb.go           # MongoDB connection & indexes
├── seed/
│   ├── seed_data.json           # Seed data
│   ├── seed_mongodb.sh          # Seed script
│   └── README.md                # Seed documentation
├── docker-compose.yml           # Docker Compose configuration
├── Dockerfile                    # Application Docker image
├── go.mod                        # Go dependencies
└── README.md                     # This file
```

## Environment Variables

- `MONGO_URI`: MongoDB connection string (default: `mongodb://localhost:27017`)
- `MONGO_DB`: Database name (default: `coupon_system`)
- `PORT`: Server port (default: `8080`)
- `GIN_MODE`: Gin framework mode (default: `release`)

## Dependencies

- `github.com/gin-gonic/gin` - HTTP web framework
- `go.mongodb.org/mongo-driver` - Official MongoDB Go driver

## License

This project is part of a technical assessment.

