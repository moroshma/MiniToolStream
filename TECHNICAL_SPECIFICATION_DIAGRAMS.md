# Диаграммы для TECHNICAL_SPECIFICATION.md

## 2.1. Общая архитектура и потоки данных

### Component Diagram (Flowchart Notation)

```mermaid
graph TB
    subgraph Clients["Клиенты"]
        PC[Publisher Client]
        SC[Subscriber Client]
    end

    subgraph MiniToolStream["MiniToolStream Platform"]
        Ingress[Ingress Service<br/>:50051]
        Egress[Egress Service<br/>:50052]

        subgraph Storage["Хранилище"]
            Tarantool[(Tarantool<br/>:3301<br/>Метаданные)]
            MinIO[(MinIO/S3<br/>:9000<br/>Payload)]
        end

        Vault[("HashiCorp Vault<br/>:8200<br/>Секреты")]
    end

    PC -->|"gRPC:<br/>Publish(data)"| Ingress
    SC -->|"gRPC:<br/>Subscribe/Fetch"| Egress

    Ingress -->|"1. Get sequence"| Tarantool
    Ingress -->|"2. Store payload"| MinIO
    Ingress -->|"3. Save metadata"| Tarantool
    Ingress -.->|"Load RSA keys"| Vault

    Egress -->|"1. Query metadata"| Tarantool
    Egress -->|"2. Fetch payload"| MinIO
    Egress -.->|"Load RSA keys"| Vault

    PC -.->|"Get JWT token"| Vault
    SC -.->|"Get JWT token"| Vault

    style Ingress fill:#e1f5ff
    style Egress fill:#fff4e1
    style Tarantool fill:#ffe1e1
    style MinIO fill:#e1ffe1
    style Vault fill:#f0e1ff
```

### Data Flow Diagram (UML 2.x Sequence Diagram)

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Ingress
    participant Tarantool
    participant MinIO
    participant Egress
    participant Consumer

    Note over Client,Consumer: Publishing Flow

    Note over Ingress,Tarantool: Step 1: Allocate sequence (BEFORE payload upload)
    Client->>Ingress: Publish(subject, data, headers)
    Ingress->>Ingress: Validate JWT (optional)
    Ingress->>Tarantool: get_next_sequence()
    Tarantool->>Tarantool: global_sequence++
    Tarantool-->>Ingress: sequence = N

    Note over Ingress,MinIO: Step 2: Upload payload FIRST (race condition prevention)
    Ingress->>Ingress: object_name = subject_N
    Ingress->>MinIO: PutObject(subject_N, data)
    alt MinIO upload fails
        MinIO-->>Ingress: Error
        Note over Ingress: Sequence N is "burned" (gap created)<br/>No metadata saved → prevents race condition
        Ingress-->>Client: Error: failed to upload data
    else MinIO upload succeeds
        MinIO-->>Ingress: OK
    end

    Note over Ingress,Tarantool: Step 3: Insert metadata AFTER payload exists
    Ingress->>Tarantool: insert_message(sequence, subject, headers, object_name)
    alt Tarantool insert fails
        Tarantool-->>Ingress: Error
        Note over Ingress: Orphaned object in MinIO<br/>(will be cleaned by TTL)
        Ingress-->>Client: Error: failed to insert metadata
    else Tarantool insert succeeds
        Tarantool-->>Ingress: OK
        Ingress-->>Client: PublishResponse(sequence=N)
    end

    Note over Client,Consumer: Subscribe Flow
    Consumer->>Egress: Subscribe(subject, durable_name)
    Egress->>Egress: Validate JWT (optional)
    Egress->>Tarantool: get_consumer_position(durable_name, subject)
    Tarantool-->>Egress: last_seq

    loop Poll for new messages
        Egress->>Tarantool: check_new_messages(subject, last_seq)
        alt New messages available
            Tarantool-->>Egress: latest_seq
            Egress-->>Consumer: Notification(sequence=latest_seq)
        end
    end

    Note over Client,Consumer: Fetch Flow (At-Least-Once with Manual ACK)
    Consumer->>Egress: Fetch(subject, durable_name, batch_size)
    Egress->>Egress: Validate JWT (optional)
    Egress->>Tarantool: get_consumer_position()
    Egress->>Tarantool: get_messages_by_subject(start_seq, limit)
    Tarantool-->>Egress: messages[]

    loop For each message
        Egress->>MinIO: GetObject(object_name)
        alt MinIO error (object not found, network failure, etc.)
            MinIO-->>Egress: Error
            Note over Egress: STOP processing batch<br/>DO NOT update consumer position<br/>(prevents message loss)
            Egress-->>Consumer: Error: failed to fetch payload
        else MinIO success
            MinIO-->>Egress: data
            Note over Egress: Position NOT updated!<br/>Consumer must ACK
            Egress-->>Consumer: Message(seq, data, headers)
        end
    end

    loop For each processed message
        Consumer->>Consumer: Process message
        Consumer->>Egress: AckMessage(durable_name, subject, sequence)
        Egress->>Tarantool: update_consumer_position(sequence)
        Tarantool-->>Egress: OK
        Egress-->>Consumer: AckResponse(success=true)
    end
```

---

## 2.2. Модель данных и API

### Class Diagram - Data Models (UML 2.x)

```mermaid
classDiagram
    class PublishRequest {
        +string subject
        +bytes data
        +map~string,string~ headers
    }

    class PublishResponse {
        +uint64 sequence
        +string object_name
        +int64 status_code
        +string error_message
    }

    class SubscribeRequest {
        +string subject
        +uint64 start_sequence
        +string durable_name
    }

    class Notification {
        +string subject
        +uint64 sequence
    }

    class FetchRequest {
        +string subject
        +string durable_name
        +int32 batch_size
    }

    class Message {
        +string subject
        +uint64 sequence
        +bytes data
        +map~string,string~ headers
        +Timestamp timestamp
    }

    class AckRequest {
        +string durable_name
        +string subject
        +uint64 sequence
    }

    class AckResponse {
        +bool success
        +string error_message
    }

    class MessageMetadata {
        <<Tarantool_Space_message>>
        +uint64 sequence PK
        +map headers
        +string object_name
        +string subject
        +uint64 create_at
    }

    class ConsumerPosition {
        <<Tarantool_Space_consumers>>
        +string durable_name PK
        +string subject PK
        +uint64 last_sequence
    }

    class Claims {
        <<JWT Claims>>
        +string client_id
        +string[] allowed_subjects
        +string[] permissions
        +RegisteredClaims
    }

    class IngressService {
        <<gRPC Service>>
        +Publish(PublishRequest) PublishResponse
    }

    class EgressService {
        <<gRPC Service>>
        +Subscribe(SubscribeRequest) stream~Notification~
        +Fetch(FetchRequest) stream~Message~
        +GetLastSequence(GetLastSequenceRequest) GetLastSequenceResponse
        +AckMessage(AckRequest) AckResponse
    }

    IngressService ..> PublishRequest : uses
    IngressService ..> PublishResponse : returns
    IngressService ..> MessageMetadata : creates
    IngressService ..> Claims : validates

    EgressService ..> SubscribeRequest : uses
    EgressService ..> Notification : streams
    EgressService ..> FetchRequest : uses
    EgressService ..> Message : streams
    EgressService ..> AckRequest : uses
    EgressService ..> AckResponse : returns
    EgressService ..> MessageMetadata : reads
    EgressService ..> ConsumerPosition : updates
    EgressService ..> Claims : validates
```

### Entity Relationship Diagram (Chen Notation)

```mermaid
erDiagram
    MESSAGE {
        uint64 sequence PK "Global unique ID"
        any headers "Message metadata"
        string object_name UK "MinIO key: subject_sequence"
        string subject FK "Topic/channel"
        uint64 create_at "Unix timestamp for TTL"
    }

    CONSUMER {
        string durable_name PK "Consumer group name"
        string subject PK "Subscribed topic (also FK to SUBJECT)"
        uint64 last_sequence "Last read message"
    }

    MINIO_OBJECT {
        string key PK "Format: subject_sequence"
        bytes data "Actual payload"
    }

    MESSAGE ||--|| MINIO_OBJECT : "object_name → key"
    MESSAGE ||--o{ CONSUMER : "subject → subject"

    MESSAGE }o--|| SUBJECT : "has"
    CONSUMER }o--|| SUBJECT : "subscribes"

    SUBJECT {
        string name PK "Topic identifier"
    }
```

---

## 2.3. Проектирование безопасности

### JWT Claims and Permissions (UML 2.x Class Diagram)

```mermaid
classDiagram
    class Claims {
        +string client_id
        +string[] allowed_subjects
        +string[] permissions
        +jwt.RegisteredClaims
        +CheckPermission(required) bool
        +CheckSubjectAccess(subject) bool
        +ValidatePublishAccess(subject) error
        +ValidateSubscribeAccess(subject) error
        +ValidateFetchAccess(subject) error
    }

    class Permission {
        <<enumeration>>
        publish
        subscribe
        fetch
        all (*)
    }

    class SubjectPattern {
        <<Wildcard_Support>>
        exact: "images.jpeg"
        prefix: "images.*"
        all: "*"
    }

    class JWTManager {
        -rsa.PrivateKey privateKey
        -rsa.PublicKey publicKey
        -string issuer
        -string vaultPath
        +GenerateToken(clientID, subjects, perms, duration) string
        +ValidateToken(token) Claims, error
        +SaveKeysToVault(ctx, client)
        +LoadKeysFromVault(ctx, client) error
    }

    Claims --> Permission : has
    Claims --> SubjectPattern : matches
    JWTManager --> Claims : creates/validates

    note for Claims "Permissions control operations:\n- publish: can send messages\n- subscribe: can receive notifications\n- fetch: can pull messages\n\nSubjects control topics:\n- 'images.*' allows 'images.jpeg'\n- '*' allows everything"
```

### Vault Secrets Structure (Flowchart Notation)

```mermaid
graph TB
    subgraph Vault["HashiCorp Vault KV v2"]
        direction TB

        JWT["secret/data/minitoolstream/jwt"]
        Tarantool["secret/data/minitoolstream/tarantool"]
        MinIO["secret/data/minitoolstream/minio"]

        subgraph JWT_Data["JWT Secrets"]
            PrivKey["private_key: RSA 2048 PEM"]
            PubKey["public_key: RSA 2048 PEM"]
        end

        subgraph Tarantool_Data["Tarantool Credentials"]
            TUser["user: minitoolstream_connector"]
            TPass["password: xxxxxxxx"]
        end

        subgraph MinIO_Data["MinIO/S3 Credentials"]
            MAccess["access_key_id: xxxxxxxx"]
            MSecret["secret_access_key: xxxxxxxx"]
        end

        JWT --> JWT_Data
        Tarantool --> Tarantool_Data
        MinIO --> MinIO_Data
    end

    Ingress[Ingress Service]
    Egress[Egress Service]
    JWTGen[jwt-gen Tool]

    Ingress -.->|read| JWT
    Ingress -.->|read| Tarantool
    Ingress -.->|read| MinIO

    Egress -.->|read| JWT
    Egress -.->|read| Tarantool
    Egress -.->|read| MinIO

    JWTGen -.->|read/write| JWT

    style JWT fill:#f0e1ff
    style Tarantool fill:#ffe1e1
    style MinIO fill:#e1ffe1
```

### Service Startup with Vault (UML 2.x Sequence Diagram)

```mermaid
sequenceDiagram
    autonumber
    participant Service as Ingress/Egress
    participant Vault as HashiCorp Vault
    participant Tarantool
    participant MinIO

    Note over Service,MinIO: Service Initialization

    Service->>Service: Load config from env/file
    Service->>Vault: Connect(address, token)
    Vault-->>Service: Connection OK

    alt JWT Auth Enabled
        Service->>Vault: Read(secret/data/minitoolstream/jwt)
        Vault-->>Service: {private_key, public_key}

        alt Keys exist
            Service->>Service: Parse RSA keys from PEM
            Service->>Service: Initialize JWTManager
        else Keys not found
            Service->>Service: Generate new RSA 2048 keypair
            Service->>Vault: Write(secret/data/minitoolstream/jwt)
            Vault-->>Service: OK
            Service->>Service: Initialize JWTManager
        end
    end

    Service->>Vault: Read(secret/data/minitoolstream/tarantool)
    Vault-->>Service: {user, password}
    Service->>Service: Update Tarantool config

    Service->>Vault: Read(secret/data/minitoolstream/minio)
    Vault-->>Service: {access_key_id, secret_access_key}
    Service->>Service: Update MinIO config

    Service->>Tarantool: Connect(address, credentials)
    Tarantool-->>Service: Connection OK
    Service->>Tarantool: Ping()
    Tarantool-->>Service: Pong

    Service->>MinIO: Connect(endpoint, credentials)
    MinIO-->>Service: Connection OK
    Service->>MinIO: EnsureBucket(bucket_name)
    MinIO-->>Service: Bucket ready

    Service->>Service: Start gRPC server
    Note over Service: ✓ Service Ready
```

---

## 2.4. Диаграмма последовательности: Publish Flow (UML 2.x)

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Ingress
    participant Auth as Auth Interceptor
    participant JWTMgr as JWTManager
    participant Tarantool
    participant MinIO

    Client->>Ingress: Publish(subject, data, headers)<br/>+ Authorization: Bearer <token>

    Note over Ingress,JWTMgr: ref: Auth Validate Fragment
    Ingress->>Auth: Intercept request
    Auth->>Auth: Extract metadata

    alt Missing Authorization Header
        Auth-->>Client: Error: Unauthenticated (missing token)
    else Token Present
        Auth->>Auth: Extract Bearer token
        Auth->>JWTMgr: ValidateToken(token)

        alt Invalid Signature
            JWTMgr-->>Auth: ErrInvalidSignature
            Auth-->>Client: Error: Unauthenticated (invalid signature)
        else Token Expired
            JWTMgr-->>Auth: ErrTokenExpired
            Auth-->>Client: Error: Unauthenticated (token expired)
        else Valid Token
            JWTMgr-->>Auth: Claims{client_id, permissions, allowed_subjects}
            Auth->>Auth: context.WithValue(ClaimsContextKey, claims)
        end
    end

    Note over Ingress: Business Logic
    Ingress->>Ingress: GetClaimsFromContext()

    alt Claims present
        Ingress->>Ingress: claims.ValidatePublishAccess(subject)

        alt No publish permission
            Ingress-->>Client: Error: PermissionDenied (insufficient permissions)
        else Subject not allowed (pattern match)
            Note over Ingress: Disabled in current implementation
        end
    end

    Note over Ingress,MinIO: Success Path - Race Condition Prevention

    Note over Ingress,Tarantool: Step 1: Allocate sequence number
    Ingress->>Tarantool: call get_next_sequence()
    Tarantool->>Tarantool: global_sequence++
    Tarantool-->>Ingress: sequence = N

    Note over Ingress,MinIO: Step 2: Upload payload FIRST
    Ingress->>Ingress: object_name = subject + "_" + sequence
    Ingress->>MinIO: PutObject(bucket, object_name, data)
    alt MinIO fails
        MinIO-->>Ingress: Error
        Note over Ingress: Sequence burned, no metadata saved
        Ingress-->>Client: Error
    else MinIO succeeds
        MinIO-->>Ingress: ETag, UploadInfo
    end

    Note over Ingress,Tarantool: Step 3: Insert metadata AFTER payload exists
    Ingress->>Tarantool: call insert_message(sequence, subject, headers, object_name)
    Tarantool->>Tarantool: insert into message space
    alt Tarantool fails
        Tarantool-->>Ingress: Error
        Note over Ingress: Orphaned object in MinIO
        Ingress-->>Client: Error
    else Tarantool succeeds
        Tarantool-->>Ingress: sequence
        Ingress-->>Client: PublishResponse{<br/>  sequence=N,<br/>  object_name="subject_N",<br/>  status_code=0<br/>}
    end
```

---

## 2.5. Диаграмма последовательности: Subscribe/Fetch Flow

### Subscribe Flow (UML 2.x Sequence Diagram)

```mermaid
sequenceDiagram
    autonumber
    participant Consumer
    participant Egress
    participant Auth as Auth Stream Interceptor
    participant JWTMgr as JWTManager
    participant Tarantool

    Consumer->>Egress: Subscribe(subject, durable_name, start_seq)<br/>+ Authorization: Bearer <token>

    Note over Egress,JWTMgr: ref: Auth Validate Fragment
    Egress->>Auth: Intercept stream

    alt JWT validation (similar to Publish)
        Auth->>JWTMgr: ValidateToken(token)
        JWTMgr-->>Auth: Claims
        Auth->>Auth: Wrap stream with authenticated context
    end

    Egress->>Egress: GetClaimsFromContext()

    alt Claims present
        Egress->>Egress: claims.ValidateSubscribeAccess(subject)

        alt No subscribe permission
            Egress-->>Consumer: Error: PermissionDenied
        end
    end

    Note over Egress,Tarantool: Initialize Subscription
    Egress->>Tarantool: call get_consumer_position(durable_name, subject)
    Tarantool-->>Egress: last_seq (or 0 if new)

    alt start_sequence provided
        Egress->>Egress: position = max(last_seq, start_sequence)
    else
        Egress->>Egress: position = last_seq
    end

    loop Polling Loop
        Note over Egress,Tarantool: Check for new messages (poll_interval)
        Egress->>Tarantool: call check_new_messages(subject, position)
        Tarantool->>Tarantool: latest_seq = get_latest_sequence_for_subject(subject)
        Tarantool-->>Egress: {has_new, latest_seq, new_count}

        alt has_new = true
            Egress-->>Consumer: stream Notification{<br/>  subject,<br/>  sequence=latest_seq<br/>}
            Egress->>Egress: position = latest_seq
        end

        Egress->>Egress: Sleep(poll_interval)

        alt Context cancelled (client disconnect)
            Egress->>Egress: Exit loop
        end
    end
```

### Fetch Flow with Manual Acknowledgment (At-Least-Once Delivery)

```mermaid
sequenceDiagram
    autonumber
    participant Consumer
    participant Egress
    participant Auth as Auth Stream Interceptor
    participant Tarantool
    participant MinIO

    Consumer->>Egress: Fetch(subject, durable_name, batch_size)<br/>+ Authorization: Bearer <token>

    Note over Egress,Auth: ref: Auth Validate Fragment
    Egress->>Auth: ValidateToken() → Claims

    Egress->>Egress: claims.ValidateFetchAccess(subject)

    alt No fetch permission
        Egress-->>Consumer: Error: PermissionDenied
    end

    Note over Egress,MinIO: Fetch Messages (WITHOUT position update)
    Egress->>Tarantool: call get_consumer_position(durable_name, subject)
    Tarantool-->>Egress: last_seq

    Egress->>Egress: start_seq = last_seq + 1
    Egress->>Tarantool: call get_messages_by_subject(subject, start_seq, batch_size)
    Tarantool->>Tarantool: SELECT from message<br/>WHERE subject = ? AND sequence >= ?<br/>LIMIT ?
    Tarantool-->>Egress: messages[] (metadata only)

    loop For each message in batch (Message Loss Prevention)
        Egress->>Egress: Extract object_name from metadata
        Egress->>MinIO: GetObject(bucket, object_name)

        alt MinIO error
            MinIO-->>Egress: Error (object not found, network issue, etc.)
            Note over Egress,Consumer: STOP processing<br/>DO NOT update position<br/>(client can retry from this point)
            Egress-->>Consumer: Error: failed to fetch payload for sequence X
        else MinIO success
            MinIO-->>Egress: data (payload bytes)
            Egress->>Egress: Merge metadata + data

            Note over Egress,Consumer: Position NOT updated automatically!<br/>Consumer MUST call AckMessage after processing
            Egress-->>Consumer: stream Message{<br/>  subject, sequence,<br/>  data, headers,<br/>  timestamp<br/>}
        end
    end

    Note over Consumer: Consumer receives messages,<br/>processes each one,<br/>then calls AckMessage

    loop For each processed message (At-Least-Once)
        Consumer->>Consumer: Process message<br/>(business logic, database writes, etc.)

        alt Processing succeeds
            Consumer->>Egress: AckMessage(durable_name, subject, sequence)<br/>+ Authorization: Bearer <token>
            Egress->>Egress: Validate JWT
            Egress->>Tarantool: call update_consumer_position(durable_name, subject, sequence)
            Tarantool->>Tarantool: UPSERT into consumers
            Tarantool-->>Egress: OK
            Egress-->>Consumer: AckResponse{success: true}
            Note over Consumer: ✓ Message acknowledged
        else Processing fails
            Note over Consumer: ⚠️ DO NOT call AckMessage<br/>Message will be redelivered<br/>on next Fetch
        end
    end

    Note over Consumer: If consumer crashes before ACK,<br/>unacknowledged messages<br/>will be redelivered (At-Least-Once)
```

---

## 2.6. Диаграмма последовательности: Auth Flow (Universal UML 2.x)

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Service as Ingress/Egress
    participant Interceptor as Unary/Stream Interceptor
    participant JWTMgr as JWTManager
    participant Handler as Business Logic

    Note over Client,Handler: Universal Authentication Fragment<br/>(Referenced by Publish/Subscribe/Fetch)

    Client->>Service: gRPC Request<br/>metadata: {authorization: "Bearer <JWT>"}
    Service->>Interceptor: Intercept(ctx, req)

    rect rgb(240, 240, 255)
        Note over Interceptor: Step 1: Extract Token
        Interceptor->>Interceptor: md = metadata.FromIncomingContext(ctx)

        alt No metadata
            Interceptor-->>Client: Error(Unauthenticated): missing metadata
        end

        Interceptor->>Interceptor: values = md.Get("authorization")

        alt No authorization header
            Interceptor-->>Client: Error(Unauthenticated): missing authorization header
        end

        Interceptor->>Interceptor: token = values[0]

        alt Not "Bearer " prefix
            Interceptor-->>Client: Error(Unauthenticated): invalid header format
        end

        Interceptor->>Interceptor: token = strings.TrimPrefix(token, "Bearer ")
    end

    rect rgb(255, 240, 240)
        Note over Interceptor,JWTMgr: Step 2: Validate Token
        Interceptor->>JWTMgr: ValidateToken(token)
        JWTMgr->>JWTMgr: jwt.ParseWithClaims(token, &Claims{}, keyFunc)
        JWTMgr->>JWTMgr: Verify signature with RSA public key

        alt Invalid signature
            JWTMgr-->>Interceptor: ErrInvalidSignature
            Interceptor-->>Client: Error(Unauthenticated): invalid signature
        else Token expired
            JWTMgr-->>Interceptor: ErrTokenExpired
            Interceptor-->>Client: Error(Unauthenticated): token expired
        else Other validation error
            JWTMgr-->>Interceptor: ErrInvalidToken
            Interceptor-->>Client: Error(Unauthenticated): invalid token
        else Valid
            JWTMgr-->>Interceptor: claims{client_id, allowed_subjects, permissions}
        end
    end

    rect rgb(240, 255, 240)
        Note over Interceptor,Handler: Step 3: Add Claims to Context
        Interceptor->>Interceptor: ctx = context.WithValue(ctx, ClaimsContextKey, claims)
        Interceptor->>Handler: handler(ctx, req)
    end

    rect rgb(255, 255, 240)
        Note over Handler: Step 4: Check Permissions
        Handler->>Handler: claims = GetClaimsFromContext(ctx)

        alt Operation = Publish
            Handler->>Handler: claims.ValidatePublishAccess(subject)
            Handler->>Handler: claims.CheckPermission("publish")

            alt No permission
                Handler-->>Client: Error(PermissionDenied): insufficient permissions
            end
        else Operation = Subscribe
            Handler->>Handler: claims.ValidateSubscribeAccess(subject)
            Handler->>Handler: claims.CheckPermission("subscribe")
        else Operation = Fetch
            Handler->>Handler: claims.ValidateFetchAccess(subject)
            Handler->>Handler: claims.CheckPermission("fetch")
        end
    end

    rect rgb(240, 255, 255)
        Note over Handler: Step 5: Check Subject Access (Optional)
        Handler->>Handler: claims.CheckSubjectAccess(subject)
        Handler->>Handler: matchSubjectPattern(pattern, subject)

        Note over Handler: Patterns:<br/>- "*" → all subjects<br/>- "images.*" → images.xxx<br/>- "exact.match" → exact only

        alt Subject not allowed
            Handler-->>Client: Error(PermissionDenied): subject access denied
        end
    end

    Handler-->>Client: Success Response / Stream
```

---

## 2.7. Диаграмма развёртывания (Flowchart Notation)

```mermaid
graph TB
    subgraph Kubernetes["Kubernetes Cluster"]
        direction TB

        subgraph IngressPod["Ingress Pod"]
            IngressApp[Ingress Service<br/>Go Binary<br/>Port: 50051]
        end

        subgraph EgressPod["Egress Pod"]
            EgressApp[Egress Service<br/>Go Binary<br/>Port: 50052]
        end

        subgraph TarantoolPod["Tarantool StatefulSet"]
            TarantoolDB[(Tarantool 2.11<br/>Port: 3301<br/>WAL + memtx)]
        end

        subgraph MinioPod["MinIO StatefulSet"]
            MinioStorage[(MinIO<br/>Port: 9000<br/>S3 API)]
        end

        subgraph VaultPod["Vault Deployment"]
            VaultApp[HashiCorp Vault<br/>Port: 8200<br/>KV v2]
        end

        subgraph Monitoring["Monitoring Stack (Optional)"]
            Prometheus[Prometheus]
            Grafana[Grafana Dashboard]
        end
    end

    subgraph External["External Clients"]
        Client1[Publisher Client]
        Client2[Subscriber Client]
    end

    Client1 -->|gRPC :50051| IngressApp
    Client2 -->|gRPC :50052| EgressApp

    IngressApp -->|gRPC :3301| TarantoolDB
    IngressApp -->|S3 API :9000| MinioStorage
    IngressApp -.->|HTTPS :8200| VaultApp

    EgressApp -->|gRPC :3301| TarantoolDB
    EgressApp -->|S3 API :9000| MinioStorage
    EgressApp -.->|HTTPS :8200| VaultApp

    IngressApp -.->|Metrics| Prometheus
    EgressApp -.->|Metrics| Prometheus
    TarantoolDB -.->|Metrics| Prometheus
    MinioStorage -.->|Metrics| Prometheus

    Prometheus -->|Dashboard| Grafana

    style IngressPod fill:#e1f5ff
    style EgressPod fill:#fff4e1
    style TarantoolPod fill:#ffe1e1
    style MinioPod fill:#e1ffe1
    style VaultPod fill:#f0e1ff
    style Monitoring fill:#f5f5f5
```

### Deployment Configuration

```yaml
# Tarantool StatefulSet
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: tarantool
spec:
  serviceName: tarantool
  replicas: 1  # Standalone mode
  template:
    spec:
      containers:
      - name: tarantool
        image: tarantool/tarantool:2.11
        ports:
        - containerPort: 3301
        volumeMounts:
        - name: data
          mountPath: /var/lib/tarantool
        - name: config
          mountPath: /opt/tarantool/init.lua
          subPath: init.lua

---
# MinIO StatefulSet
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: minio
spec:
  serviceName: minio
  replicas: 1
  template:
    spec:
      containers:
      - name: minio
        image: minio/minio:latest
        args: ["server", "/data"]
        ports:
        - containerPort: 9000

---
# Ingress Service Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minitoolstream-ingress
spec:
  replicas: 2  # Horizontal scaling
  template:
    spec:
      containers:
      - name: ingress
        image: minitoolstream/ingress:latest
        ports:
        - containerPort: 50051
        env:
        - name: VAULT_ADDR
          value: "http://vault:8200"
        - name: VAULT_TOKEN
          valueFrom:
            secretKeyRef:
              name: vault-token
              key: token

---
# Egress Service Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minitoolstream-egress
spec:
  replicas: 2  # Horizontal scaling
  template:
    spec:
      containers:
      - name: egress
        image: minitoolstream/egress:latest
        ports:
        - containerPort: 50052
        env:
        - name: VAULT_ADDR
          value: "http://vault:8200"
```

---

## 2.8. Диаграмма компонентов (Flowchart Notation)

```mermaid
graph TB
    subgraph MiniToolStreamIngress["MiniToolStream Ingress"]
        direction TB
        IngressGRPC[gRPC Handler<br/>IngressService]
        IngressAuth[Auth Interceptor<br/>JWT Validation]
        IngressUC[Publish UseCase]
        IngressMinioRepo[MinIO Repository]
        IngressTarantoolRepo[Tarantool Repository]
        IngressLogger[Logger<br/>Zap]
        IngressConfig[Config Loader<br/>Env/Vault]

        IngressGRPC --> IngressAuth
        IngressAuth --> IngressUC
        IngressUC --> IngressMinioRepo
        IngressUC --> IngressTarantoolRepo
        IngressUC --> IngressLogger
        IngressConfig --> IngressGRPC
    end

    subgraph MiniToolStreamEgress["MiniToolStream Egress"]
        direction TB
        EgressGRPC[gRPC Handler<br/>EgressService]
        EgressAuth[Auth Stream Interceptor<br/>JWT Validation]
        EgressUC[Message UseCase]
        EgressMinioRepo[MinIO Repository]
        EgressTarantoolRepo[Tarantool Repository]
        EgressLogger[Logger<br/>Zap]
        EgressConfig[Config Loader<br/>Env/Vault]

        EgressGRPC --> EgressAuth
        EgressAuth --> EgressUC
        EgressUC --> EgressMinioRepo
        EgressUC --> EgressTarantoolRepo
        EgressUC --> EgressLogger
        EgressConfig --> EgressGRPC
    end

    subgraph SharedLib["MiniToolStreamConnector Library"]
        direction TB
        AuthModule[Auth Module<br/>JWT Manager<br/>Claims<br/>Permissions]
        ModelProto[Protobuf Models<br/>Publish/Subscribe/Fetch]
        ConnectorLib[Client Library<br/>Publisher<br/>Subscriber]

        ModelProto --> ConnectorLib
        AuthModule --> ConnectorLib
    end

    subgraph Infrastructure["Infrastructure Components"]
        direction LR
        TarantoolLua[(Tarantool<br/>init.lua<br/>Spaces & Functions)]
        MinioS3[(MinIO<br/>S3-compatible<br/>Object Storage)]
        VaultKV[(HashiCorp Vault<br/>KV v2<br/>Secrets Manager)]
    end

    IngressAuth -.->|uses| AuthModule
    IngressGRPC -.->|implements| ModelProto
    IngressMinioRepo -->|S3 API| MinioS3
    IngressTarantoolRepo -->|gRPC| TarantoolLua
    IngressConfig -.->|load secrets| VaultKV

    EgressAuth -.->|uses| AuthModule
    EgressGRPC -.->|implements| ModelProto
    EgressMinioRepo -->|S3 API| MinioS3
    EgressTarantoolRepo -->|gRPC| TarantoolLua
    EgressConfig -.->|load secrets| VaultKV

    ConnectorLib -->|gRPC calls| IngressGRPC
    ConnectorLib -->|gRPC calls| EgressGRPC

    style MiniToolStreamIngress fill:#e1f5ff
    style MiniToolStreamEgress fill:#fff4e1
    style SharedLib fill:#e1ffe1
    style Infrastructure fill:#ffe1e1
```

### Component Dependencies (Flowchart Notation)

```mermaid
graph LR
    subgraph Layer1["Application Layer"]
        Publisher[Publisher Client]
        Subscriber[Subscriber Client]
    end

    subgraph Layer2["Library Layer"]
        Connector[MiniToolStreamConnector]
        Auth[Auth Module]
        Model[Protobuf Models]
    end

    subgraph Layer3["Service Layer"]
        Ingress[Ingress Service]
        Egress[Egress Service]
    end

    subgraph Layer4["Repository Layer"]
        TarantoolRepo[Tarantool Repository]
        MinIORepo[MinIO Repository]
    end

    subgraph Layer5["Storage Layer"]
        Tarantool[(Tarantool)]
        MinIO[(MinIO)]
        Vault[(Vault)]
    end

    Publisher --> Connector
    Subscriber --> Connector
    Connector --> Auth
    Connector --> Model

    Connector --> Ingress
    Connector --> Egress

    Ingress --> TarantoolRepo
    Ingress --> MinIORepo
    Ingress --> Auth

    Egress --> TarantoolRepo
    Egress --> MinIORepo
    Egress --> Auth

    TarantoolRepo --> Tarantool
    MinIORepo --> MinIO
    Auth -.->|load keys| Vault
    Ingress -.->|load config| Vault
    Egress -.->|load config| Vault

    style Layer1 fill:#e1f5ff
    style Layer2 fill:#e1ffe1
    style Layer3 fill:#fff4e1
    style Layer4 fill:#ffe1e1
    style Layer5 fill:#f0e1ff
```

---

## Итоговая таблица компонентов

| Компонент | Технология | Порт | Назначение |
|-----------|-----------|------|------------|
| **MiniToolStream Ingress** | Go 1.24, gRPC | 50051 | Прием сообщений (Publish) |
| **MiniToolStream Egress** | Go 1.24, gRPC | 50052 | Выдача сообщений (Subscribe/Fetch) |
| **Tarantool** | Tarantool 2.11, Lua | 3301 | Хранение метаданных, consumer positions |
| **MinIO** | MinIO (S3-compatible) | 9000 | Хранение payload (больших данных) |
| **HashiCorp Vault** | Vault KV v2 | 8200 | Управление секретами (JWT keys, credentials) |
| **MiniToolStreamConnector** | Go library | N/A | Клиентская библиотека + Auth модуль |
| **Publisher Client** | Go application | N/A | Пример клиента для публикации |
| **Subscriber Client** | Go application | N/A | Пример клиента для подписки |
| **jwt-gen** | Go CLI tool | N/A | Генерация JWT токенов |

---

## Схемы данных

### Tarantool Space: message

| Field | Type | Index | Description |
|-------|------|-------|-------------|
| sequence | uint64 | PRIMARY | Глобальный уникальный ID сообщения |
| headers | any (map) | - | Метаданные сообщения (msgpack) |
| object_name | string | - | Ключ в MinIO: `{subject}_{sequence}` |
| subject | string | subject<br/>subject_sequence | Топик/канал |
| create_at | uint64 | create_at | Unix timestamp для TTL |

### Tarantool Space: consumers

| Field | Type | Index | Description |
|-------|------|-------|-------------|
| durable_name | string | PRIMARY (composite) | Имя consumer group |
| subject | string | PRIMARY (composite)<br/>subject | Подписанный топик |
| last_sequence | uint64 | - | Последний прочитанный sequence |

### Vault Secret Paths

| Path | Fields | Description |
|------|--------|-------------|
| `secret/data/minitoolstream/jwt` | `private_key`, `public_key` | RSA 2048 ключи для JWT |
| `secret/data/minitoolstream/tarantool` | `user`, `password` | Учетные данные Tarantool |
| `secret/data/minitoolstream/minio` | `access_key_id`, `secret_access_key` | Учетные данные MinIO |
