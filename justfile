test-e2e-localnet test="":
    cd localnet && go test -v . -tags=e2e -run={{test}} -timeout=30m
