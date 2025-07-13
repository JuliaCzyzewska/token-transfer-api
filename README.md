# Token Transfer API


## Description
The Token Transfer API is a GraphQL service built in Go that simulates token transactions between Ethereum-like wallet addresses. It uses a PostgreSQL database and supports high-precision balances and transfers using the `NUMERIC(28,18)` type. <br>

The API provides two core functionalities:
- Querying wallet balances
- Transferring tokens between wallets




## Requirements
To run the application, ensure you have the following installed:
* Docker Engine
* Docker Compose


## How to run the app
### Build the Docker images:

```bash
docker-compose build 
```

### Start API server:

```bash
docker compose up app
```

The server will be available at:    http://localhost:8080 <br>
This includes the GraphQL Playground.

### Stop the server:
```bash
# In the terminal running the server:
CTRL+C

# Then clean up containers:
docker compose down
```

### Run tests:
```bash
docker compose up test
```




## Schema overview
#### Types:
```graphql
type Wallet {
  address: ID!
  balance: String!
}
```

#### Queries:
```graphql
wallet(address: ID!): Wallet
```

#### Mutations:
```graphql
transfer(from_address: ID!, to_address: ID!, amount: String!): String!
```





## Query example
#### Query: 
```json
{
  wallet(address: "0x0000000000000000000000000000000000000000") {
    address
    balance
  }
}
```

#### Response: 
```json
{
  "data": {
    "wallet": {
      "address": "0x0000000000000000000000000000000000000000",
      "balance": "1000000.000000000000000000"
    }
  }
}
```


## Mutations examples

### Standard transfer
#### Mutation:
```json
mutation {
  transfer(
    from_address: "0x0000000000000000000000000000000000000000",
    to_address: "0xA000000000000000000000000000000000000000",
    amount: "145.678900"
  )
}
```

#### Response:
```json
{
  "data": {
    "transfer": "999854.321100000000000000"
  }
}
```

### Smallest transfer available
#### Mutation:
```json
mutation {
  transfer(
    from_address: "0xA000000000000000000000000000000000000000",
    to_address: "0xB000000000000000000000000000000000000000",
    amount: "0.000000000000000001"
  )
}
```


#### Response:
```json
{
  "data": {
    "transfer": "145.678899999999999999"
  }
}
```


## Wallet Creation:
*  At startup, the database is seeded with a single wallet - 
  address `0x0000000000000000000000000000000000000000` - holding a balance of 1,000,000 BTP tokens.

* A sender must already exist in the database; otherwise, the transfer is rejected.

*  If the recipient address is not found during transfer, it will be automatically created.


## Testing
The test suite connects to a real PostgreSQL database connection to mimic production-like behavior.  
To prevent interference with the main application data, tests use a separate database table:

- Production server uses: `wallets`
- Tests use: `test_wallets`

This separation is enabled by dependency injection in `resolvers.go`, which allows the table name to be dynamically configured for different environments (production vs testing).



## Constraints
The API enforces the following constraints and behaviors:

#### Token precision:
* All balances and transfer amounts use PostgreSQL's `NUMERIC(28,18)` for high precision.


* Minimum Transfer Amount: Transfers must be greater than 0 and must fit within the allowed `NUMERIC(28,18)` precision.

#### Balance safety:
* Transactions that would cause a wallet’s balance to go negative are rejected.


#### Address rules:
* Format: All addresses must follow the Ethereum hexadecimal format: they must start with `0x` and be followed by exactly 40 hexadecimal characters. EIP-55 checksum is not required. Addresses are treated as case-insensitive.
* Sender existence: Transfers cannot originate from addresses that do not exist in the database. The sender wallet must already be registered.
* Distinct addresses: Transfers must be made between two different addresses. It is not allowed to transfer tokens from an address to itself.

#### Concurrency:
* Advisory Locks: PostgreSQL advisory locks prevent concurrent race conditions by locking hashed wallet addresses in a consistent order.

#### Transactions safety
*  All operations are done within a transaction; on error, the state is rolled back entirely.


