## Prerequisites

- **Docker** (or Docker Engine + Docker Compose)
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

   # Claim a coupon
   curl -X POST http://localhost:8080/api/coupons/claim \
     -H "Content-Type: application/json" \
     -d '{"user_id": "user_123", "coupon_name": "FLASH_SALE_2026"}'

   # Get coupon details
   curl http://localhost:8080/api/coupons/FLASH_SALE_2026
   ```

5. **Tests**
```
go test -v ./tests"
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


### 1. Claim Coupon

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

### 2. Get Coupon Details

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

## Environment Variables

- `MONGO_URI`: MongoDB connection string (default: `mongodb://localhost:27017`)
- `MONGO_DB`: Database name (default: `coupon_system`)
- `PORT`: Server port (default: `8080`)
- `GIN_MODE`: Gin framework mode (default: `debug`) for local development


### Architecture 
1) To Ensure that only one coupon is being used per customer, i am implementing the 
`createIndex` with the customer id and the coupon id as the constraint, this prevents insertion under any circumstances ( this requires that the index be created first though, it is present in the mongodb.go file. )
Also, making use of the `$setOnInsert` feature
2) To ensure that there are no race condition type errors, the insertions are being done using atomic operations. Claims are created before stock is decremented. If stock decrement fails, the claim is rolled back via a compensating delete.