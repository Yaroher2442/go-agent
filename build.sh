
GOOS=linux go build -ldflags "-linkmode 'external' -extldflags '-static'"
mv main pca