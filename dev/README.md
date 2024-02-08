# Random notes on developing for SusQL Operator

- Operator build has been verified on RHEL and Ubuntu.
- docker.io and quay.io have been used.
- Sample steps to build and push image.

```
export IMG=REGISTRYURL/REPOSITORYNAME/susql_operator:latest
podman login 
make build && make docker-build 
podman push ${IMG}
```

When performing deploy besure to set and export `SUSQL_REGISTRY` and `SUSQL_IMAGE_NAME` to values used above.

e.g.,

