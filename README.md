# Go_NaiveCoin
Thanks to lhartikk for his naivecoin tutorial: [https://lhartikk.github.io/](https://lhartikk.github.io/)

This repository follows the tutorial and reimplemented by golang, original NodeJs version is [https://github.com/lhartikk/naivecoin](https://github.com/lhartikk/naivecoin).

## Installation

### Requirements

Go_NaiveCoin requires `Go` 1.10+. To install `Go`, follow this [link](https://golang.org/doc/install). 

In addition, [dep](https://github.com/golang/dep) is required to manage dependencies. 

### Getting the source

Clone the repo into $GOPATH/src/github.com/:

```
git clone http://github.com/Ksloveyuan/go_naivecoin.git go_naivecoin 
cd go_naivecoin
```

Install dependencies:

```
dep ensure
```

### Building

To build the main app, just run

```
make build
```

To run tests, run

```
make test
```

To clear build artifacts,
```
make clean
```