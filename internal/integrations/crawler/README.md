# Crawler integration

Crawler and third-party data are an independent integration, not the Assets
module's core data model. An explicit authorized import is required before
third-party content becomes an Assets record or project-library ProjectAssetRef.

The YouShu adapter in this package owns only the product/CID GraphQL wire DTOs,
structured protocol errors, and conservative request gating. It has no default
remote endpoint, never stores or logs session material, and does not download
media. Tests replay synthetic, versioned response fixtures through an injected
HTTP transport; business state and persistence remain owned by Insights and
Assets.
