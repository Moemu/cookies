# 米云裂变交接 manifest（cookies draft v1）

`miyun-handoff-manifest/cookies-draft-v1` 是 cookies 本地导出格式。AI 系统的
STEP 01 目前只确认可人工上传文件，尚未确认本格式或任何运行时 API；因此本格式
不得被视作跨系统契约，也不会触发 API 调用、事件发布或对方状态变更。

ZIP 固定包含以下目录和文件：

```text
manifest.csv
viral/source/
product/media/
product/docs/
```

`manifest.csv` 始终以 UTF-8 BOM 开头，使用 RFC 4180 CSV 转义。每个字段是字符串；
缺失值为空字符串。字段顺序固定如下：

```text
manifest_version,handoff_id,handoff_version,source_material_name,source_file,
miyun_material_id,source_url,source,delivery_days,cumulative_impressions,
related_ads,related_creators,target_product,target_category,product_media_files,
product_document_files,notes,juliang_spend,parameter_version,input_hash
```

`source_file`、`product_media_files` 和 `product_document_files` 是 ZIP 内的相对路径；
文件名经过服务端清洗、保留名处理和去重。`input_hash` 覆盖冻结的源 AssetVersion、
已确认 profile 版本、产品资料引用、manifest 版本与参数版本。

一次 handoff 可以冻结多个已确认且已入库的源素材。`source_file`、
`source_material_name`、`miyun_material_id`、`source_url` 及对应指标仍保持字符串类型；
多值按冻结的稳定顺序以分号连接，所有视频都位于 `viral/source/`。该约定仍仅为 cookies
draft，不构成 AI 系统契约。

`exported` 仅表示 ZIP 成功生成并流出，绝不表示 AI 已收到。`delivered` 必须由操作人员
在 cookies 中显式确认。
