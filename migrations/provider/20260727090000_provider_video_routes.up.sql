ALTER TABLE provider_connections
  DROP CHECK chk_provider_connection_type,
  ADD CONSTRAINT chk_provider_connection_type
    CHECK (connection_type IN ('adapter_gateway', 'ark'));
