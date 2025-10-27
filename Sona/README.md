## Sona iOS App - Session Storage

This document explains how the app stores and uses session-related data.

### What is stored

- server_user_uid: The backend-generated user UID returned by GET /get_user. Persisted immediately on flow start.
- data.address: Grid account address (if available, typically after OTP).
- data.grid_user_id: Grid user id (if available, typically after OTP).
- data.authentication.*: Optional authentication payload when available from Grid (not required for basic app features).

The session payload is persisted as JSON in the iOS Keychain under a single record. The storage is append/merge-only for known keys, ensuring previously stored values are retained when new fields arrive later in the flow.

### Where it is stored

- Keychain service: "Sona.Keychain"
- Key: "auth_session_json"

### Lifecycle

1) Start flow (GET /get_user): The app decodes `user_uid` and saves it as `server_user_uid` immediately.
2) OTP verify (POST /grid/otp_verify or /grid/auth_otp_verify): The app merges `data.address` and `data.grid_user_id` into the stored session.
3) Reading session: `SessionManager` exposes computed properties for the most commonly used fields.
4) Clearing session: Settings -> Clear Session deletes the Keychain item.

### Accessors

- SessionManager.shared.serverUserUID
- SessionManager.shared.gridUserId
- SessionManager.shared.gridAddress

### Display

The `SettingsView` shows:
- Grid Address
- Grid User ID
- Server User UID
- Parent ID (if known via AppStorage)


