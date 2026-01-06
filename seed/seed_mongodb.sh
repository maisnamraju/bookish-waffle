#!/bin/bash

# MongoDB Seed Script for Flash Sale Scenario
# This script seeds MongoDB with test data for the coupon system

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
MONGO_HOST="${MONGO_HOST:-localhost}"
MONGO_PORT="${MONGO_PORT:-27017}"
MONGO_DB="${MONGO_DB:-coupon_system}"
MONGO_COLLECTION_COUPONS="coupons"
MONGO_COLLECTION_CLAIMS="claims"
SEED_DATA_FILE="${SCRIPT_DIR}/seed_data.json"

echo "🌱 Seeding MongoDB database: $MONGO_DB"
echo "📍 Host: $MONGO_HOST:$MONGO_PORT"
echo "📁 Seed file: $SEED_DATA_FILE"
echo ""

# Check if seed data file exists
if [ ! -f "$SEED_DATA_FILE" ]; then
    echo "❌ Error: Seed data file not found: $SEED_DATA_FILE"
    exit 1
fi

# Check if mongoimport is available
if ! command -v mongoimport &> /dev/null; then
    echo "❌ Error: mongoimport not found. Please install MongoDB Database Tools."
    echo "   On macOS: brew install mongodb-database-tools"
    echo "   On Ubuntu: sudo apt-get install mongodb-database-tools"
    exit 1
fi

# Import coupon data from seed_data.json
echo "📦 Importing coupon data from $SEED_DATA_FILE..."
mongoimport \
  --host "$MONGO_HOST:$MONGO_PORT" \
  --db "$MONGO_DB" \
  --collection "$MONGO_COLLECTION_COUPONS" \
  --file "$SEED_DATA_FILE" \
  --jsonArray \
  --drop \
  --quiet

# Create indexes
echo "🔧 Creating indexes..."
mongosh "$MONGO_HOST:$MONGO_PORT/$MONGO_DB" --quiet --eval "
  // Create unique index on coupon name
  db.$MONGO_COLLECTION_COUPONS.createIndex({ name: 1 }, { unique: true });
  
  // Create unique compound index on claims (user_id, coupon_id)
  // This prevents double-dip attacks
  db.$MONGO_COLLECTION_CLAIMS.createIndex(
    { user_id: 1, coupon_id: 1 },
    { unique: true, name: 'user_coupon_unique' }
  );
  
  // Create index on coupon_id for faster lookups
  db.$MONGO_COLLECTION_CLAIMS.createIndex({ coupon_id: 1 });
  
  // Create index on coupon_name for querying
  db.$MONGO_COLLECTION_CLAIMS.createIndex({ coupon_name: 1 });
  
  print('✅ Indexes created successfully');
"

echo ""
echo "✅ Seeding completed!"
echo ""
echo "📊 Verification:"
mongosh "$MONGO_HOST:$MONGO_PORT/$MONGO_DB" --quiet --eval "
  const coupons = db.$MONGO_COLLECTION_COUPONS.find().toArray();
  print('📋 Imported Coupons:');
  coupons.forEach(coupon => {
    print('   • ' + coupon.name + ': ' + coupon.remaining_amount + ' / ' + coupon.amount + ' (Active: ' + coupon.is_active + ')');
  });
  print('');
  const flashSale = db.$MONGO_COLLECTION_COUPONS.findOne({ name: 'FLASH_SALE_2024' });
  if (flashSale) {
    print('✅ Flash Sale Coupon Verified:');
    print('   Name: ' + flashSale.name);
    print('   Stock: ' + flashSale.remaining_amount + ' / ' + flashSale.amount);
    print('   Active: ' + flashSale.is_active);
  } else {
    print('❌ Flash Sale coupon not found!');
  }
"

