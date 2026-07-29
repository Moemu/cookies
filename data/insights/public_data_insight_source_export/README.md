# Public Data Insight Source Export

This directory stores deployable sample data for the material insight demo.

The original export at `/Users/bytedance/Downloads/public_data_insight_source_export` currently contains the app code and loaders, but no CSV or media files. The repository keeps a small CSV with the same schema so deployments can show example data immediately. Replace or add CSV files in this directory when the full export data is available.

Server import behavior:

- The MVP API reads `data/insights/public_data_insight_source_export/*.csv` by default.
- Set `PUBLIC_INSIGHT_DATA_DIR` to override the data directory in deployment.
- Optional local playable videos can be placed under `downloads/douyin/<item_id>.mp4` inside this directory.
