ALTER TABLE connector_platform_objects
  DROP CHECK chk_connector_platform_object_kind,
  ADD CONSTRAINT chk_connector_platform_object_kind CHECK (
    object_kind IN ('image_material', 'video_material', 'orange_landing_page')
  );
