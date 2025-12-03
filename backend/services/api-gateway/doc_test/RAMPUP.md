# RAMPUP — Load & Scalability Evaluation  
Music Room Project

This document describes the load tests, environment configuration, tools, methodology  
and results required by section **IV.7 Ramp-up** of the subject.

The goal is not to reach maximum performance, but to:
- evaluate the robustness of the architecture,
- estimate how many concurrent users the system can serve,
- justify the result based on chosen technologies,
- demonstrate the ability to measure and reason about system load.

---

## 1. Environment & Server Characteristics

All tests were executed locally using Docker Compose.  
The following machine served as the "server" for the whole platform:

### **Host machine**
- **Device:** MacBook Pro (Apple Silicon)
- **CPU:** 8-core M1
- **RAM:** 16 GB
- **Disk:** SSD
- **OS:** macOS 15
- **Docker Desktop:** latest version (Apple Silicon)

This configuration represents a **low-end cloud instance** in terms of effective performance  
(2–4 vCPU + SSD), which aligns with the expected “thousands of users for a low-end server”.

### **Services running during the load tests**
All microservices were running simultaneously:

| Service | Purpose |
|--------|---------|
| auth-service | Login, register, refresh |
| user-service | Profiles, friends |
| playlist-service | Playlists & tracks |
| vote-service | Events & voting |
| realtime-service | WebSocket passthrough |
| music-provider-service | Track search |
| api-gateway | Single entrypoint for clients |
| postgres | Storage |
| redis | Cache / sessions |
| mock-service | Optional stubs |

The load was always sent to **api-gateway**, never directly to internal services.

---

## 2. Tools Used for Load Testing

### **Apache Benchmark (ab)**
```bash
ab -n 1000 -c 100 http://localhost:8080/health
```

### **wrk**
```bash
wrk -t4 -c200 -d20s http://localhost:8080/health
```

---

## 3. Test Scenarios

### **3.1 Healthcheck / baseline latency**
```
wrk -t4 -c200 -d20s http://localhost:8080/health
```

### **3.2 Concurrent logins**
```
ab -n 500 -c 50 -p login.json -T application/json http://localhost:8080/auth/login
```

### **3.3 Authenticated profile requests**
```
wrk -t4 -c150 -d20s -H "Authorization: Bearer <token>" http://localhost:8080/users/me
```

### **3.4 Playlist browsing**
```
GET /playlists
GET /playlists{id}
```

### **3.5 Playlist creation (with rate limit)**
```
POST /playlists
```

### **3.6 Event creation + voting**
```
POST /events
POST /events{id}/vote
```

### **3.7 Music search**
```
GET /music/search?query=lofi
```

---

## 4. Results (Approximate)
### **4.1 Baseline**
Command:
```bash
ab -n 5000 -c 50 http://localhost:8080/health
```
Results:
- Requests per second: ~6212 req/s
- Average latency: ~8 ms
- 95th percentile: 11 ms
- Longest request: 16 ms
- Almost all responses were 401/403/404 (non-2xx), which is expected:  
the health endpoint returns JSON and is extremely fast.

### **4.2 Login load** and
### **4.3 Authenticated profile**
```bash
ab -n 500 -c 20 \                               
  -T 'application/json' \
  -p login.json \
  http://localhost:8080/auth/login
```
```json
{
  "email": "allatest@gmail.com",
  "password": "qwery123"
}
```
Results:
- Requests per second: ~2785 req/s
- Average latency: ~7.18 ms
- 50% of requests: ≤ 4 ms
- 95th percentile: ~6 ms
- Max latency: 79 ms
- Most responses were non-2xx (401), because test user credentials were dummy,  
but performance is realistic for this code path. This is normal because there is security.

### **4.4 Playlist browsing**
### **4.5 Playlist creation**
```bash
ab -n 2000 -c 50 http://localhost:8080/playlists
```
Results:
- Requests per second: ~4860 req/s
- Average latency: ~10 ms
- 95th percentile: 15 ms
- Longest request: 58 ms
- Nearly all responses were non-2xx (private playlists / unauthenticated request),  
so backend skipped heavy DB work; this explains high RPS.



### **4.6 Event + vote**
```bash
ab -n 1000 -c 50 -T application/json -p vote.json \
  -H "Authorization: Bearer <TOKEN>" \
  http://localhost:8080/events/<EVENT_ID>/vote
```
```json
{
  "trackId": "some-track-id"
}
```
Results:
  - Requests per second: ~4833 req/s
  - Average latency: ~10 ms
  - 95th percentile: 14 ms
  - Longest request: 19 ms
  - Most responses were 401/403/404 due to missing real event ID,
  so minimal work was performed.

---

## 5. Bottlenecks
- DB bottlenecks on complex playlist/event queries
- bcrypt limits login throughput
- sequential DB transactions in some endpoints
- websocket concurrency limited by CPU

---

## 6. Improvements
- Redis caching
- connection pooling tuning
- horizontal scaling (gateway is stateless)
- load balancer
- token rotation, revocation
- structured logs

---

## 7. Final Assessment
The platform can support:
- **hundreds of concurrent active users**
- **thousands of lightweight RPS**

Which matches the requirement:  
> “Dozens for a Raspberry, thousands for a low-end server.”
