# Furyctl

## Install

Get the right binary for you:

```bash
wget https://s3.wasabisys.com/sighup-releases/linux/latest/furyctl
chmod +x furyctl
mv furyctl /usr/local/bin
```

Available endpoints are built as follows:

`https://s3.wasabisys.com/sighup-releases/{arch}/{version}/furyctl`

Supported architectures are (64 bit):
- `linux`
- `darwin`

Current availability versions are:
- `v0.1.0`
- `latest`

## Usage

```bash
furyctl
├── init : Downloads Furyfile.yml and kustomization.yaml from the distribution repository to the current directory.
├── vendor : Downloads Fury packages specified inside Furyfile.yaml
├── help
└── version
```

Write a [`Furyfile.yml`](Furyfile.yml) in the root of your project and then simply run `furyctl vendor`.
It will download packages specified inside `Furyfile.yaml` to the `vendor` directory.

## Contributing

We still use `go mod` as the golang package manager. Once you have that installed you can run `$ go mod vendor` 
and `$ go build` or `$ go install` should run without problems.
