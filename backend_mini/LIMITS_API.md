# App Limits API

This API manages time limits for apps on kids' devices, allowing parents to set daily usage limits and fees for extending those limits.

## Set Limit

**Endpoint:** `POST /set_limit`

**Description:** Creates or updates an app limit for a specific parent-child relationship.

**Request Body:**
```json
{
  "parent_email": "parent@example.com",
  "kid_email": "kid@example.com",
  "app": "com.example.app",
  "time_per_day": 60,
  "fee_extra_hour": "1000000"
}
```

**Parameters:**
- `parent_email` (string, required): Email of the parent setting the limit
- `kid_email` (string, required): Email of the child whose usage is being limited
- `app` (string, required): Bundle identifier or name of the app to limit
- `time_per_day` (integer, required): Daily time limit in minutes (must be > 0)
- `fee_extra_hour` (string, required): Fee in EURC micro-units to extend limit by 1 hour

**Response:**
```json
{
  "limit_id": "abc123def456",
  "parent_email": "parent@example.com",
  "kid_email": "kid@example.com",
  "app": "com.example.app",
  "time_per_day": 60,
  "fee_extra_hour": 1000000,
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Note:** If a limit already exists for the same parent-child-app combination, it will be updated with the new values. The `created_at` timestamp remains unchanged on updates.

## Get Limits

**Endpoint:** `POST /get_limits`

**Description:** Retrieves all app limits for a specific child.

**Request Body:**
```json
{
  "kid_email": "kid@example.com"
}
```

**Parameters:**
- `kid_email` (string, required): Email of the child whose limits to retrieve

**Response:**
```json
[
  {
    "limit_id": "abc123def456",
    "parent_email": "parent@example.com",
    "kid_email": "kid@example.com",
    "app": "com.example.app",
    "time_per_day": 60,
    "fee_extra_hour": 1000000,
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "limit_id": "xyz789ghi012",
    "parent_email": "parent@example.com",
    "kid_email": "kid@example.com",
    "app": "com.another.app",
    "time_per_day": 120,
    "fee_extra_hour": 2000000,
    "created_at": "2024-01-15T11:00:00Z"
  }
]
```

**Note:** Results are ordered by creation date (most recent first). Returns an empty array if no limits exist for the child.

## Authentication

All endpoints require Bearer token authentication with the value: `SonaBetaTestAPi`

Include the Authorization header:
```
Authorization: Bearer SonaBetaTestAPi
```

## Error Responses

All endpoints return standard error responses:

```json
{
  "error": "error message description"
}
```

Common error scenarios:
- `400 Bad Request`: Missing required fields, invalid data types, or invalid values
- `500 Internal Server Error`: Database errors or other server-side issues

## Integration Notes

Based on the TimeLimitLogic.md specification:

1. **Parent sets limits**: Parent app calls `/set_limit` to configure app usage restrictions
2. **Kid retrieves limits**: Kid app calls `/get_limits` to fetch all limits configured for them
3. **Backend as source of truth**: The backend stores the policy (limits, prices for extensions)
4. **Enforcement happens on-device**: The kid's iOS app uses DeviceActivity to enforce limits locally
5. **Pay to extend**: When a kid hits a limit, they can pay the `fee_extra_hour` amount to extend usage by 1 hour

The backend does not push enforcement commands to devices. Instead, it acts as the policy source that kid devices sync and enforce locally using iOS Screen Time APIs.

