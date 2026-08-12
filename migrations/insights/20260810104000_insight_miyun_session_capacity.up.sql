-- A complete browser Cookie request header can exceed the old 4 KiB envelope.
-- AES-GCM adds 28 bytes, so the service limits plaintext to 16 KiB minus that
-- overhead before writing this 16 KiB ciphertext column.
ALTER TABLE insight_miyun_connections
  MODIFY COLUMN session_ciphertext VARBINARY(16384) NOT NULL;
