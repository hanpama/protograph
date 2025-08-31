# protograph — GraphQL-gRPC bridge

## Introduction

protograph turns a declarative, directive-annotated GraphQL SDL into:
- A single, directive-free GraphQL schema (stitched and validated)
- A deterministic set of `.proto` files (services + messages projected from SDL)
- A bridge server that executes GraphQL by calling your gRPC backends on-the-fly

No resolver layer and no generated application code required.

## Why protograph
- Schema-as-source: SDL is the only source of truth. No handwritten resolvers.
- Predictable mapping: directives map 1:1 to protobuf constructs; outcomes are inferable from SDL.
- Operational performance: automatic depth-wise batching eliminates N+1; gRPC is low-latency and binary.

## How it works
1) Parse and validate your GraphQL project (a modular tree of `.graphql` files).
2) Apply protograph directives to build an intermediate representation (IR).
3) Emit `.proto` files from the IR and stitch a directive-free GraphQL schema.
4) Run the GraphQL server, which translates GraphQL operations into gRPC calls at runtime, batching where possible.

The internal executor performs breadth‑first execution: it drains synchronous fields immediately, batches all async fields once per depth, completes values per GraphQL spec, and propagates Non‑Null violations correctly.

## Quickstart (local demo)
Requirements: Go, `protoc`, and the `protoc-gen-go` and `protoc-gen-go-grpc` plugins on PATH.

- Generate sample `.proto` from the test schema and compile Go stubs:
  - `./run generate-test-proto`
- Start the sample gRPC backend:
  - `./run simple-grpc-server`
- In another terminal, run the GraphQL bridge gateway:
  - `./run simple-graphql-server`
- Query the gateway at `http://localhost:8081/graphql`.

## CLI
Use `protograph` to compile SDL, generate `.proto`, or run the GraphQL gateway.

- Serve (GraphQL ↔ gRPC bridge):
  - `protograph serve -graphql.root <dir> -graphql.rootpkg <name> -transport.backend "*=host:port" -server.addr ":8080"`
- Compile SDL (validate + stitch):
  - `protograph compile-sdl -graphql.root <dir> -graphql.rootpkg <name> -out schema.graphql`
- Compile `.proto` files:
  - `protograph compile-proto -graphql.root <dir> -graphql.rootpkg <name> -out ./out`

Common `serve` flags:
- `-server.metadata-header X-User-ID` forward an HTTP header to gRPC metadata (repeatable)
- `-transport.backend <ServiceFullName=host:port>` map a gRPC service to an endpoint (repeatable); use `*=` as wildcard default
- `-transport.rpc-timeout 3s`, `-transport.max-conns-per-endpoint 2`
- `-graphql.introspection true|false`

## Authoring your SDL
Add directives to describe how data is loaded and resolved. protograph compiles the SDL into protobuf services/messages and uses them at runtime without generated resolvers.

- `@loader` (OBJECT): declare how an object can be loaded (single or compound key; optional batching)
- `@id` (FIELD): mark identifier fields; used implicitly for default mappings
- `@load` (FIELD): resolve via a loader on the target type (no field arguments)
- `@resolve` (FIELD): resolve via an explicit RPC (batching optional)
- `@internal` (FIELD): server-only field; removed from GraphQL but present in protobuf messages
- `@mapScalar` (SCALAR): map a custom scalar to a protobuf scalar

Example:
```graphql
# user.graphql
"User domain"
type User @loader {               # implicit key from @id
  id: ID! @id
  name: String!
  email: String! @internal       # hidden from GraphQL, present in proto
}

# post.graphql
"Blog posts"
type Post @loader(keys: ["id"]) {
  id: ID! @id
  title: String!
  authorId: ID! @id @internal
  author: User @load(with: { id: "authorId" })  # via User loader
  views: Int! @resolve(with: { id: "id" }, batch: true)  # explicit, batched
}

# root.graphql (base)
type Query { _empty: String }

# Extend roots from feature files
extend type Query {
  post(id: ID!): Post
  user(id: ID!): User
}
```

Service layout tips:
- One gRPC service per `.graphql` file; file path becomes protobuf package (e.g., `account/user/profile.graphql` → `package account.user;`).
- Keep the dependency graph acyclic (DAG). When referencing a foreign type, define reverse relationships via `extend` on the referencing side to avoid cycles.

## Protobuf projection (at a glance)
- Field numbers: hash‑based allocation with reserved ranges and linear probing; deterministic across builds.
- Naming: `Load<Type>By<Key>` / `BatchLoad…` for loaders; `Resolve<Type><Field>` / `BatchResolve…` for resolvers; snake_case fields.
- Requests:
  - Loader: loader keys only
  - Implicit resolver: GraphQL args + all parent `@id` fields (including `@internal @id`)
  - Explicit resolver: GraphQL args + fields from `with` mapping (or default to all parent `@id` if omitted)
- Root fields: non‑batched RPCs by default

## Observability
- Optional OpenTelemetry export (`-otel.endpoint` and `-otel.service`) when running `serve`.

## Where to go next
- Full specification: see the section below
- Executor semantics: `internal/executor/doc.go`
- Try the sample app under `tests/simple`

---

# SDL IR Reference

## 1 Directive Reference

### 1.1 `@loader` (OBJECT)

Declares a key by which an object can be loaded. Multiple loaders can be defined for alternative loading patterns.

```graphql
directive @loader(
  key:  String,          # single-key form (mutually exclusive with keys)
  keys: [String!],       # multi-key form (mutually exclusive with key)
  batch: Boolean = true  # generate Batch* if true, Load* if false
) repeatable on OBJECT
```

**Key Resolution (for parameterless @loader only):**
1. If fields marked with `@id` exist: use all `@id` fields as compound key
2. Else if field named `id` exists: use `id` as single key  
3. Else: compilation error

**Example: Single Key Loader**
```graphql
type UserSingle @loader(key: "email") {
  id: String!
  email: String!
}
# Generates: BatchLoadUserSingleByEmail
```

**Example: Compound Key Loader**
```graphql
type PostComposite @loader(keys: ["authorId", "date"]) {
  id: String!
  authorId: String!
  date: String!
}
# Generates: BatchLoadPostCompositeByAuthorIdDate (alphabetically sorted)
```

**Example: Default Key Resolution**
```graphql
type ProductDefault @loader {
  productId: String! @id
  sku: String! @id
}
# Generates: BatchLoadProductDefaultByProductIdSku (all @id fields)
```

**Duplicate Handling:** Multiple @loader declarations with identical keys result in compilation error.

### 1.2 `@id` (FIELD)

Marks a field as an identifier automatically included in implicit resolver requests.

```graphql
directive @id on FIELD_DEFINITION
```

**Example: Explicit vs Implicit @id**
```graphql
type ExplicitId {
  userId: String! @id  # explicit @id
  id: String!          # just a regular field
}

type ImplicitId {
  id: String!          # implicit @id (no explicit @id present)
  name: String!
}
```

### 1.3 `@load` (FIELD)

Resolves the field by calling a loader defined on the target type.

```graphql
directive @load(
  with: JSON!  # maps parent fields to loader keys
) on FIELD_DEFINITION
```

**Example: Valid @load**
```graphql
type Article {
  authorId: String!
  author: User @load(with: { id: "authorId" })  # ✅ Valid
  # comments(limit: Int): [Comment!] @load(...)  # ❌ Error: @load cannot have arguments
}
```

### 1.4 `@resolve` (FIELD)

Explicitly invokes a dedicated gRPC method for this field.

```graphql
directive @resolve(
  with:  JSON,            # maps parent fields to request
  batch: Boolean = false   # generate Batch* if true, Resolve* if false
) on FIELD_DEFINITION
```

**Batching Override:** Implicit resolvers always have `batch: false`. To enable batching, use explicit `@resolve(with: {...}, batch: true)`.

**Default mapping when `with` is omitted:** Include all parent `@id` fields with identical names in the request (equivalent to `{ idField: "idField" }` for each `@id`). This mirrors implicit resolver composition.

**Example: Root Field (Implicit)**
```graphql
type Query {
  currentUser: User  # Implicit @resolve(with: {}, batch: false)
}
# Generates: ResolveQueryCurrentUser (no batch)
```

**Example: Implicit vs Explicit Resolver**
```graphql
type Blog {
  id: String! @id
  
  # Implicit resolver: batch: false
  posts(limit: Int): [Post!]!
  # Generates: ResolveBlogPosts
  
  # Explicit resolver without with: defaults to include @id
  details: BlogDetails @resolve

  # Explicit resolver with batch
  followerCount: Int! @resolve(with: { blogId: "id" }, batch: true)
  # Generates: BatchResolveBlogFollowerCount
}
```

### 1.5 `@internal` (FIELD)

Marks a field as server-only. Removed from GraphQL schema but included in protobuf messages.

```graphql
directive @internal on FIELD_DEFINITION
```

**Example: Internal Identifier**
```graphql
type Account {
  id: String! @id
  internalRef: String! @internal @id  # Hidden but sent to resolvers
}
```

### 1.6 `@mapScalar` (SCALAR)

Maps a custom scalar to a protobuf scalar type.

```graphql
directive @mapScalar(
  toProtobuf: String! = "string"
) on SCALAR
```

---

## 2 Module, Package, and Service Layout

### 2.1 File-to-Service Mapping

- Each `.graphql` file → one gRPC service named `<PascalCaseFilename>Service`
- File path → protobuf package (e.g., `account/user/profile.graphql` → `package account.user;`)
- All types in a file belong to that service

### 2.2 Service Dependency Graph

**Definition:** File A depends on File B when A references a top-level type defined in B.

**Rule:** The service dependency graph must be a DAG (Directed Acyclic Graph) — no cycles allowed.

**Reverse References:** All reverse relationships to foreign types must be owned by the referencing service using `extend`:

**Example: Breaking Cycles with extend**
```graphql
# ❌ WRONG - Creates cycle
# user.graphql:
type User {
  authoredPosts: [Post!]!  # User → Post dependency
}

# post.graphql:
type Post {
  author: User!  # Post → User dependency
}

# ✅ CORRECT - No cycle
# user.graphql:
type User {
  id: String!
  name: String!
  # No reference to Post
}

# post.graphql:
type Post {
  author: User!  # Post → User dependency
}

extend type User {
  authoredPosts: [Post!]!  # Reverse relationship owned by Post service
}
```

**Note:** All foreign type reverse relationships are owned by the service that initiates the reference, not by the foreign type's service.

### 2.3 Root Type Pattern

**Recommended Structure:**
```graphql
# root.graphql - Base definitions only
type Query {
  _empty: String
}

type Mutation {
  _empty: String  
}

# user.graphql - Extend root types
extend type Query {
  user(id: ID!): User
  users: [User!]!
}

extend type Mutation {
  createUser(input: CreateUserInput!): User!
}
```

**Reason:** Defining root types in multiple services creates multi-directional dependencies.

---

## 3 Protobuf Projection

### 3.1 Field-number Allocation

**Algorithm:**
1. Compute FNV-1a 32-bit hash of GraphQL field name
2. `candidate = (hash % 31767) + 1` (range: 1-31767, never 0)
3. If candidate in `[19000-19999]`: linear probe
4. If collision: linear probe (increment, wrap at 31767 to 1)
5. If all numbers exhausted: compilation error

**Example: Field Number Assignment**
```graphql
 type HashExample {
   id: String!        # hash("id") % 31767 + 1 = 14233
   name: String!      # hash("name") % 31767 + 1 = 7823
   reserved: String!  # hash("reserved") % 31767 + 1 = 19500 → 19501 → ... → 19999 (all reserved) → 20000 (first free after reserved block)
 }
```

| Message Type | Numbering | Fields Included |
|-------------|-----------|-----------------|
| `<Type>Source` | Hash-based | Non-`@load`/`@resolve` fields |
| `*Request` | Hash-based | See composition rules below |
| `*Response` | Sequential | `data = 1` |
| `Batch*Request` | Sequential | `batches = 1` |
| `Batch*Response` | Sequential | `batches = 1` |

**Request Composition:**
- **LoaderRequest**: loader key fields only
- **Implicit ResolverRequest**: GraphQL arguments + all `@id` fields (including `@internal @id`)
- **Explicit ResolverRequest**: GraphQL arguments + `with`-mapped parent fields; if `with` omitted, include all parent `@id` fields by default

### 3.2 Naming Rules

- CamelCase → snake_case (`authorId` → `author_id`)
- Single key: `Load<Type>By<Key>`
- Compound keys: `Load<Type>By<Key1><Key2>...` (alphabetically sorted, CamelCase preserved)
- Resolvers: `Resolve<Type><Field>` / `BatchResolve<Type><Field>`

### 3.3 Scalar Mapping

| GraphQL | Protobuf |
|---------|----------|
| Int | int32 |
| Float | double |
| String | string |
| Boolean | bool |
| ID | string |
| Custom | Per `@mapScalar` (default: string) |

### 3.4 Interface & Union

No protograph directives on Interface/Union types or their fields. Only concrete types may use directives.

### 3.5 Root Type Projection

- No `QuerySource`/`MutationSource` messages
-
Each root field → non-batched RPC (implicit `batch: false`)

### 3.6 Object Source Field Projection

**Example: Source Message Generation**
```graphql
type FieldExample {
  id: String! @id           # ✅ In Source
  secret: String! @internal # ✅ In Source
  computed: Int! @resolve(with: {})  # ❌ Not in Source
  author: User @load(with: { id: "authorId" })  # ❌ Not in Source
  title: String!            # ✅ In Source
}
```

Generated `FieldExampleSource`:
```proto
message FieldExampleSource {
  string id = 14233;
  string secret = 8923;
  string title = 28020;
}
```

### 3.7 Enum Projection

**Example: Enum with Hash Collision**
```graphql
enum Color { 
  RED     # hash("RED") % 31767 + 1 = 5823
  BLUE    # hash("BLUE") % 31767 + 1 = 5823 → 5824 (collision, probed)
}
```

Generates:
```proto
enum Color {
  COLOR_UNSPECIFIED = 0;
  COLOR_RED = 5823;
  COLOR_BLUE = 5824;  // Probed due to collision
}
```

---

## 4 Deprecation Semantics

Standard GraphQL `@deprecated` directive supported. Combined with `@internal`: field removed from GraphQL (internal takes precedence).

---

## 5 Bridge-server Runtime

### 5.1 Batching Semantics

**Example: Batch vs Non-Batch**
```graphql
type Analytics {
  id: String! @id
  
  # Non-batched (implicit)
  dailyViews(date: String): Int!
  
  # Batched (explicit)
  weeklyViews(week: Int): Int! @resolve(with: { id: "id" }, batch: true)
}
```

Runtime behavior:
- `dailyViews`: immediate RPC per request
- `weeklyViews`: aggregated within execution depth

---

## 6 Validation Rules

**Example: Common Validation Errors**
```graphql
# ❌ Error: @load cannot have arguments
type Invalid1 {
  author(id: String): User @load(with: { id: "id" })
}

# ❌ Error: @loader without key and no id field
type Invalid3 @loader {
  name: String!
}

# ❌ Error: 'with' references non-existent field
type Invalid4 {
  related: Thing @load(with: { id: "nonExistent" })
}
```
