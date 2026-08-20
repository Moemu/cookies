-- Ocean Engine Connector is a distinct read-only web_api source.
-- This forward migration expands existing Insights checks without changing history.
ALTER TABLE insight_data_sources
  DROP CHECK chk_insight_data_sources_platform,
  ADD CONSTRAINT chk_insight_data_sources_platform CHECK (platform IN ('douyin', 'kuaishou', 'xiaohongshu', 'wechat', 'tencent_ads', 'ocean_engine', 'other')),
  DROP CHECK chk_insight_data_sources_ingest_mode,
  ADD CONSTRAINT chk_insight_data_sources_ingest_mode CHECK (ingest_mode IN ('api', 'service_account', 'file_import', 'computer_use', 'web_api', 'business'));

ALTER TABLE insight_metric_daily
  DROP CHECK chk_insight_metric_daily_kind,
  ADD CONSTRAINT chk_insight_metric_daily_kind CHECK (platform_object_kind IN ('creative', 'ad', 'account', 'project', 'promotion', 'material'));
