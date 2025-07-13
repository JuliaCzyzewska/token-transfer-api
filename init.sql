CREATE TABLE wallets (
    address TEXT PRIMARY KEY,
    token_balance NUMERIC(28,18) NOT NULL CHECK (token_balance >= 0)
);

CREATE TABLE test_wallets (
    address TEXT PRIMARY KEY,
    token_balance NUMERIC(28,18) NOT NULL CHECK (token_balance >= 0)
);

INSERT INTO wallets (address, token_balance)
VALUES ('0x0000000000000000000000000000000000000000', 1000000);

INSERT INTO test_wallets (address, token_balance)
VALUES ('0x0000000000000000000000000000000000000000', 1000000);
