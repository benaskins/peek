bin := "peek"
prefix := env_var_or_default("PREFIX", env_var("HOME") + "/.local")
install_dir := prefix + "/bin"

default: build

build:
    go build -o {{bin}} .

test:
    go test ./...

fmt:
    go fmt ./...

vet:
    go vet ./...

clean:
    rm -f {{bin}}

install: build
    mkdir -p {{install_dir}}
    install -m 0755 {{bin}} {{install_dir}}/{{bin}}
    @echo "installed {{install_dir}}/{{bin}}"

uninstall:
    rm -f {{install_dir}}/{{bin}}
