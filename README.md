# Furyctl

## Install
Get the right binary for you:
```
wget https://s3.wasabisys.com/sighup-releases/linux/latest/furyctl
chmod +x furyctl
mv furyctl /usr/local/bin
```
Available endpoints are built as follow:

`https://s3.wasabisys.com/sighup-releases/{arch}/{version}/furyctl`

Supported architectures are (64 bit):
- `linux`
- `darwin`

Current availability versions are: 
- `v0.1.0`
- `latest`

## Usage

```
furyctl
├── install : looks for a Furyfile (default `./Furyfile.yaml`) and download it's content`
├── printDefault : prints a Furyfile example
├── help
└── version
```

Write a [`Furyfile.yml`](Furyfile.yml) in the root of your project and then simply run `furyctl install`.
It will download what you have specified in the `vendor` directory in your project root.



## Contributing
We still use `go mod` as golang package manager. Once you have that installed you can run `go mod vendor` and `go build` or `go install` should run without problems

