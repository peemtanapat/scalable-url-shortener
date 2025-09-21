# Redis Web Console Access Guide

## Redis Commander Web Interface

I've added Redis Commander to your Docker Compose setup. This provides a web-based interface to manage and monitor your Redis instance.

### Access Information

- **URL**: http://localhost:8088
- **Username**: admin
- **Password**: admin

### Features Available

- Browse Redis keys and values
- Monitor the `url_counter` key
- Execute Redis commands
- View database statistics
- Real-time monitoring

### Starting the Services

```bash
docker-compose up -d
```

### Accessing the Console

1. Open your browser
2. Navigate to http://localhost:8088
3. Login with admin/admin
4. You'll see your Redis database with the `url_counter` key

### Monitoring Your URL Counter

- Look for the key `url_counter` in the database
- You can see its current value
- Watch it increment as you create short URLs

## Alternative Options

### Option 1: Redis CLI (Command Line)

```bash
# Connect to Redis container
docker exec -it redis-counter redis-cli

# Check the current counter value
GET url_counter

# Monitor all commands in real-time
MONITOR
```

### Option 2: RedisInsight (Desktop Application)

- Download from: https://redis.com/redis-enterprise/redis-insight/
- Connect to: localhost:6379
- More advanced features and better UI

### Option 3: Redis Desktop Manager

- Third-party GUI application
- Available for Windows/Mac/Linux
- Connect to: localhost:6379

## Useful Redis Commands for Your URL Shortener

```bash
# Check current counter value
GET url_counter

# Set counter to specific value (if needed)
SET url_counter 1000

# Increment counter manually
INCR url_counter

# Get all keys
KEYS *

# Monitor real-time commands
MONITOR
```

## Security Note

The Redis Commander interface uses basic authentication (admin/admin). In production, you should:

1. Use strong passwords
2. Restrict network access
3. Consider using Redis AUTH
4. Use HTTPS if exposing publicly
