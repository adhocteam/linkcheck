linkcheck
=========

linkcheck takes a root URL and recurses down through the links it finds in the
HTML pages, checking for broken links (HTTP 404).

Usage
-----

``` shell
$ linkcheck -root https://adhocteam.us/
```

Installation
------------

Requires [Go](https://golang.org/) to be installed.

``` shell
$ go get github.com/adhocteam.us/linkcheck
```

Building RPM
------------

Requires docker and a valid github personal access token and username

``` shell
  $ docker build --build-arg git_access_token=access_token --build-arg git_user_name=username -t linkchecker .
  $ docker run -it linkchecker
  $ rpm -qip linkchecker-latest.rpm

```

License
-------

See [COPYING](./COPYING).
