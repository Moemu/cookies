# 米云裂变交接 manifest（cookies draft v2）

`miyun-handoff-manifest/cookies-draft-v2` 是 cookies 的本地、版本化导出格式。
它仅描述人工导入所需的文件与可追溯性，不是 AI 系统的已确认契约，也不会调用外部 API、发布事件或改变对方状态。

ZIP 固定布局：

```text
manifest.csv
viral/source/
product/media/
product/docs/
```

`manifest.csv` 使用 UTF-8 BOM 和 RFC 4180 CSV 转义。字段顺序固定为：

```text
manifest_version,handoff_id,handoff_version,source_material_id,source_file,
product_media_files,product_document_files,parameter_version,input_hash
```

每个冻结的源视频恰好对应一条数据行，因此 `source_material_id` 与
`source_file` 是一对一关系。产品媒体和文档路径在每行重复，以便导入端无需依赖
额外状态即可解析完整包；空列表写为空字符串。文件名、路径和重复名均由服务端控制。

`input_hash` 覆盖冻结的源 AssetVersion、已确认 profile 版本、产品资料引用、
manifest 版本与参数版本。`cookies-draft-v1` 仅用于已冻结的历史 handoff；不得重写。
