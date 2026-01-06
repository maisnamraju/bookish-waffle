import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const successRate = new Rate('successful_claims');
const conflictRate = new Rate('conflict_errors');
const noStockRate = new Rate('no_stock_errors');

export const options = {
  stages: [
    { duration: '0s', target: 50 }, // Ramp up to 50 users immediately
    { duration: '5s', target: 50 }, // Stay at 50 users for 5 seconds
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'], // 95% of requests should be below 2s
    http_req_failed: ['rate<0.1'], // Error rate should be less than 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const COUPON_NAME = __ENV.COUPON_NAME || 'FLASH_SALE_2024';

export default function () {
  // Generate unique user ID for each virtual user
  const userId = `user_${__VU}_${__ITER}`;
  
  const payload = JSON.stringify({
    user_id: userId,
    coupon_name: COUPON_NAME,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const response = http.post(`${BASE_URL}/coupons/claim`, payload, params);

  // Check response status and categorize results
  const isSuccess = check(response, {
    'status is 200': (r) => r.status === 200,
  });

  const isConflict = check(response, {
    'status is 409': (r) => r.status === 409,
  });

  const isNoStock = check(response, {
    'status is 400 (no stock)': (r) => r.status === 400 && r.body.includes('no stock'),
  });

  // Track metrics
  successRate.add(isSuccess);
  conflictRate.add(isConflict);
  noStockRate.add(isNoStock);

  // Log response for debugging
  if (response.status === 200) {
    console.log(`✅ User ${userId} successfully claimed coupon`);
  } else if (response.status === 409) {
    console.log(`⚠️ User ${userId} - already claimed`);
  } else if (response.status === 400 && response.body.includes('no stock')) {
    console.log(`❌ User ${userId} - no stock available`);
  } else {
    console.log(`❌ User ${userId} - unexpected status: ${response.status}, body: ${response.body}`);
  }

  sleep(0.1); // Small delay between requests
}

export function handleSummary(data) {
  const successCount = data.metrics.successful_claims.values.count;
  const conflictCount = data.metrics.conflict_errors.values.count;
  const noStockCount = data.metrics.no_stock_errors.values.count;
  
  return {
    'stdout': `
========================================
Flash Sale Attack Test Results
========================================
Total Requests: ${data.metrics.http_reqs.values.count}
Successful Claims: ${successCount}
Conflict Errors: ${conflictCount}
No Stock Errors: ${noStockCount}
Expected: 5 successful claims, rest should be no stock errors
========================================
    `,
  };
}

