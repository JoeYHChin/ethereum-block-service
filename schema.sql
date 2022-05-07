CREATE TABLE eth_block(
	block_num BIGINT UNSIGNED PRIMARY KEY,
	block_hash VARCHAR(66),
	block_time BIGINT UNSIGNED,
	parent_hash VARCHAR(66),
	block_stable TINYINT UNSIGNED,
	INDEX(block_stable)
);

CREATE TABLE block_transaction(
	block_num BIGINT UNSIGNED NOT NULL,
	tx_hash VARCHAR(66) PRIMARY KEY,
	tx_from VARCHAR(42),
	tx_to VARCHAR(42),
	tx_nonce BIGINT UNSIGNED,
	tx_data MEDIUMBLOB,
	tx_value BIGINT UNSIGNED,
	FOREIGN KEY(block_num) REFERENCES eth_block(block_num) ON DELETE CASCADE,
	INDEX(block_num)
);

CREATE TABLE transaction_log(
	block_num BIGINT UNSIGNED NOT NULL,
	tx_hash VARCHAR(66),
	log_index INT UNSIGNED,
	log_data BLOB,
	PRIMARY KEY(tx_hash, log_index),
	FOREIGN KEY(block_num) REFERENCES eth_block(block_num) ON DELETE CASCADE,
	INDEX(block_num)
);