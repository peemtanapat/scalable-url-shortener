# Redis Integration for URL Shortener

## Overview

The URL shortener now uses Redis as a global counter to generate unique IDs for the `generateShortCode` function. This ensures that each shortened URL gets a unique, auto-incrementing ID.

## Changes Made

### 1. Dependencies

- Added `github.com/redis/go-redis/v9` to Go module dependencies

### 2. Redis Client Setup

- Added Redis client initialization with configurable connection
- Environment variable `REDIS_URL` for Docker deployment (defaults to `localhost:6379` for local development)
- Connection testing on startup

### 3. Global Counter Implementation

- `getNextID()` function uses Redis `INCR` command on key `url_counter`
- Auto-incrementing ensures unique IDs across all instances
- Error handling for Redis connection failures

### 4. Integration with Short Code Generation

- Modified POST `/api/v1/urls` endpoint to fetch ID from Redis instead of hardcoded value
- Proper error responses for Redis failures

### 5. Docker Configuration

- Added Redis service to `docker-compose.yml`
- Redis runs on port 6379 with data persistence
- Environment variable `REDIS_URL=redis:6379` for container communication

## Running the Application

### Local Development

1. Start Redis locally:

   ```bash
   redis-server
   ```

2. Run the Go application:
   ```bash
   cd convert-api
   go run main.go
   ```

### Docker Deployment

```bash
docker-compose up --build
```

## Redis Key Usage

- `url_counter`: Auto-incrementing counter for unique IDs

## Benefits

1. **Unique IDs**: No collisions between different instances
2. **Scalability**: Multiple API instances can share the same counter
3. **Persistence**: Counter survives application restarts
4. **Performance**: Redis INCR is atomic and fast
