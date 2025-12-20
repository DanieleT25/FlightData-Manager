# FlightData Manager - API Demo & Walkthrough

This document guides you through the main usage flows of the **FlightData Manager** system. `restish` will be used to simulate client calls to the API Gateway (NGINX).

**Prerequisites:**

- System started with `docker-compose up -d`
- SSL certificates generated in `pkg/certs/`
- Tool installed: [Restish](https://rest.sh/)

## 1. Security and Redirection (HTTP -> HTTPS)

The system enforces the use of HTTPS. If a client attempts to contact the HTTP port (3080), NGINX responds with a `308` Redirect to the secure port (3443).
Since the certificates were auto-generated, the verification that restish applies to every call must be disabled using the `--rsh-insecure` command.

**Comando (HTTP):**: Let's verify that the insecure call is redirected.

```bash
curl -I http://localhost:3080/docs/user
```

**Expected Result:**

```http
HTTP/1.1 308 Permanent Redirect
Server: nginx/1.29.3
Date: Fri, 19 Dec 2025 17:35:31 GMT
Content-Type: text/html
Content-Length: 171
Connection: keep-alive
Location: https://localhost:3443/docs/user
```

## 2. User Registration
Let's create two different users.

### A. User 1 Creation (Test1)
We use a specific `X-Request-ID` (101) to test idempotency later.

```bash
restish --rsh-insecure POST https://localhost:3443/users \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: 101" \
  '{
    "email": "test1@test.it",
    "password": "Password123!",
    "first_name": "Test1",
    "last_name": "Test",
    "card_number": "1234567812345678",
    "expiration_date": "12/30",
    "cvv": "123"
  }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 200 OK
Connection: keep-alive
Content-Length: 288
Content-Type: application/json
Date: Fri, 19 Dec 2025 17:41:33 GMT
Link: </schemas/UserOutputBody.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/UserOutputBody.json"
  user: {
    card_details: {
      card_number: "1234567812345678"
      cvv: "123"
      expiration_date: "12/30"
    }
    email: "test1@test.it"
    first_name: "Test1"
    last_name: "Test"
    password_hash: ""
    registered_at: "2025-12-19T17:41:33.465370889Z"
  }
}
```

The system guarantees that a request with the same `X-Request-ID` (coming from the same IP) is not processed twice but returns the original response cached in Redis. Consequently, if we run the command again, the result of the JSON response will not change.

### B. User 2 Creation (Test2)

```bash
restish --rsh-insecure POST https://localhost:3443/users \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: 102" \
  '{
    "email": "test2@test.it",
    "password": "Password123!",
    "first_name": "Test2",
    "last_name": "Test",
    "card_number": "1234567812345678",
    "expiration_date": "12/30",
    "cvv": "123"
  }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 200 OK
Connection: keep-alive
Content-Length: 288
Content-Type: application/json
Date: Fri, 19 Dec 2025 17:49:07 GMT
Link: </schemas/UserOutputBody.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/UserOutputBody.json"
  user: {
    card_details: {
      card_number: "1234567812345678"
      cvv: "123"
      expiration_date: "12/30"
    }
    email: "test2@test.it"
    first_name: "Test2"
    last_name: "Test"
    password_hash: ""
    registered_at: "2025-12-19T17:49:07.281658918Z"
  }
}
```

### C. Attempt to recreate an existing user (different request id)

```bash
restish --rsh-insecure POST https://localhost:3443/users \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: 103" \
  '{
    "email": "test2@test.it",
    "password": "Password123!",
    "first_name": "Test2",
    "last_name": "Test",
    "card_number": "1234567812345678",
    "expiration_date": "12/30",
    "cvv": "123"
  }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 409 Conflict
Connection: keep-alive
Content-Length: 118
Content-Type: application/problem+json
Date: Fri, 19 Dec 2025 17:52:22 GMT
Link: </schemas/ErrorModel.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/ErrorModel.json"
  detail: "User already exists"
  status: 409
  title: "Conflict"
}
```

### D. Attempt to recreate an existing user (different IP address)

```bash
restish --rsh-insecure POST https://localhost:3443/users \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: 102" \
  -H "X-Forwarded-For: 1.2.3.4" \
  '{
    "email": "test2@test.it",
    "password": "Password123!",
    "first_name": "Test2",
    "last_name": "Test",
    "card_number": "1234567812345678",
    "expiration_date": "12/30",
    "cvv": "123"
  }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 409 Conflict
Connection: keep-alive
Content-Length: 118
Content-Type: application/problem+json
Date: Fri, 19 Dec 2025 17:56:28 GMT
Link: </schemas/ErrorModel.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/ErrorModel.json"
  detail: "User already exists"
  status: 409
  title: "Conflict"
}
```


## 3. User Deletion

### A. Deletion of a user with correct credentials
```bash
restish --rsh-insecure DELETE https://localhost:3443/users/test1@test.it \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: 104" \
    '{ "password": "Password123!" }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 204 No Content
Connection: keep-alive
Date: Fri, 19 Dec 2025 18:06:08 GMT
Server: nginx/1.29.3
```

Repeating the command, as with insertion, the result will not change.

### B. Deletion of a non-existent user

```bash
restish --rsh-insecure DELETE https://localhost:3443/users/test1@test.it \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: 105" \
    '{ "password": "Password123!" }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 404 Not Found
Connection: keep-alive
Content-Length: 114
Content-Type: application/problem+json
Date: Fri, 19 Dec 2025 18:10:16 GMT
Link: </schemas/ErrorModel.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/ErrorModel.json"
  detail: "User not found"
  status: 404
  title: "Not Found"
}
```

### C. Deletion of a user with incorrect credentials

```bash
restish --rsh-insecure DELETE https://localhost:3443/users/test2@test.it \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: 106" \
    '{ "password": "pippo" }'
```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 401 Unauthorized
Connection: keep-alive
Content-Length: 119
Content-Type: application/problem+json
Date: Fri, 19 Dec 2025 18:12:35 GMT
Link: </schemas/ErrorModel.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/ErrorModel.json"
  detail: "Invalid password"
  status: 401
  title: "Unauthorized"
}
```

## 4. Setting Interests

Now let's configure the airports to monitor for test2.

```bash
restish --rsh-insecure POST https://localhost:3443/interests \
  -H 'Content-Type: application/json' \
  -H 'email: test2@test.it' \
  -H 'password: Password123!' \
  '{
    "interests": [
      {
        "airport_code": "LICJ",
        "low_value": 2,
        "high_value": 10
      },
      {
        "airport_code": "LIRF",
        "low_value": 5,
        "high_value": 50
      }
    ]
  }'

```

**Expected Result:**

```http
WARN: Disabling TLS security checks
HTTP/1.1 200 OK
Connection: keep-alive
Content-Length: 110
Content-Type: application/json
Date: Fri, 19 Dec 2025 18:25:00 GMT
Link: </schemas/SetInterestsOutputBody.json>; rel="describedBy"
Server: nginx/1.29.3

{
  $schema: "http://localhost/schemas/SetInterestsOutputBody.json"
  message: "Interests updated successfully"
}
```
Let's restart the data-collector container with the command `docker-compose restart data-collector`; upon restart, it will download data regarding the airports of interest.
