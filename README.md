# Tusk⚡
[![Go](https://github.com/15jgme/tusk/actions/workflows/go.yml/badge.svg)](https://github.com/15jgme/tusk/actions/workflows/go.yml)

A lightweight and (eventually) pretty cli tool for updating docker container instances. Written in Go using BubbleTea.

If you're deploying containers onto a single server and don't want to remember which ports go with which image when you pull new images and restart your containers, this tool is for you.

![image](resources/tuskDemo.gif)

## Install
- **With a go installation**

  Run: `go install github.com/15jgme/tusk@latest`
- **Without go**

    Not available at the moment. See [issue #4](https://github.com/15jgme/tusk/issues/4)
    
If you encounter an issue with your docker api version, please set your 'DOCKER_API_VERSION' environment variable to the version suggested in the error message.
