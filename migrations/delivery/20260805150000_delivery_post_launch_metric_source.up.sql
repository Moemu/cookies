ALTER TABLE delivery_metric_snapshots
  DROP CHECK chk_delivery_metric_source,
  ADD CONSTRAINT chk_delivery_metric_source
    CHECK (source IN ('demo_fixture', 'post_launch_simulator'));
