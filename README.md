# PetConnect

PetConnect is now a client-server Flutter application rather than an in-memory UI demo. The implemented functional core persists users, pets, social content, interactions, swipes, matches, chats, communities, events, adoption listings, notifications, and refresh-token sessions in PostgreSQL. Redis backs realtime tickets/pub-sub, MinIO stores uploaded media, and Flutter connects through REST and WebSockets.

## Prerequisites

- Flutter with Dart 3.12 or newer
- Go 1.24 or newer
- Docker Desktop with Docker Compose

## Run it on Windows

Open PowerShell in the project directory.

1. Start the dependencies and API. Leave this terminal open:

   ```powershell
   .\scripts\run-api.ps1
   ```

2. In a second terminal, install Flutter packages and run the client:

   ```powershell
   flutter pub get
   flutter run -d chrome
   ```

The web client uses `http://localhost:8080/v1` automatically. An Android emulator automatically uses `http://10.0.2.2:8080/v1`:

```powershell
flutter run -d emulator-5554
```

For a physical phone, use your computer's LAN address:

```powershell
flutter run --dart-define=PETCONNECT_API_URL=http://192.168.1.20:8080/v1
```

Replace `192.168.1.20` with the computer's address and allow port 8080 through the local firewall.

## Demo account

```text
Email:    demo@petconnect.local
Password: PetConnect123!
```

The login screen is prefilled with this account. You can also select “New here? Create an account”; registration, onboarding, and the first pet are written to PostgreSQL.

## Local services

| Service | Address |
| --- | --- |
| API | http://localhost:8080 |
| API readiness | http://localhost:8080/health/ready |
| PostgreSQL/PostGIS | localhost:5433 |
| Redis | localhost:6380 |
| MinIO API | http://localhost:9000 |
| MinIO console | http://localhost:9001 |

MinIO's local username/password are `petconnect` / `petconnect_dev_secret`.

Stop the foreground API with `Ctrl+C`, then stop the dependency containers:

```powershell
docker compose down
```

`docker compose down -v` also deletes all PetConnect local database and uploaded-media volumes. Use it only when you intentionally want a completely clean database.

## Entire stack in Docker

The API also has a multi-stage container build:

```powershell
Copy-Item .env.example .env
docker compose up --build
```

Then run the Flutter client in a second terminal. The foreground Go workflow above is faster for day-to-day development and supports hot iteration.

## Useful checks

```powershell
flutter analyze
flutter test

Set-Location server
go test ./...
```

## Implemented end-to-end

- Email/password registration and login
- Short-lived JWT access tokens and rotated opaque refresh tokens
- Secure refresh-token storage in the Flutter client
- Owner profile and multi-pet CRUD APIs
- PostGIS nearby pet discovery and filters
- Persistent feed, posts, media uploads, likes, saves, and comments
- Stories and reels APIs
- Idempotent swipes, reciprocal matching, and automatic chat creation
- Persistent membership-protected chat and read state
- Single-use Redis WebSocket tickets and realtime match/message events
- Communities, memberships, events/RSVPs, adoption listings/saves
- Notifications and read state
- Structured API errors, CORS allowlist, security headers, ownership checks, and upload limits

Google/Apple sign-in, push providers, payment-provider billing, video transcoding, and production cloud credentials require external provider accounts and are intentionally not represented as fake working buttons. The backend boundaries are ready for those integrations.
