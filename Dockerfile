FROM centos
# Install golang
RUN yum -y install rubygems ruby-devel ruby-json gcc gcc-c++ python-setuptools rpm-build openssh-clients git make libffi-devel
RUN /bin/bash -c 'curl https://s3.amazonaws.com/aws-cli/awscli-bundle.zip -o awscli-bundle.zip; unzip awscli-bundle.zip'
RUN /bin/bash -c 'awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws'
RUN /bin/bash -c 'curl -LO https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz'
RUN /bin/bash -c 'tar -C /usr/local -xvzf go1.7.linux-amd64.tar.gz'
RUN /bin/bash -c 'mkdir -p /projects/{bin,pkg,src}'
RUN /bin/bash -c 'gem install fpm'
ARG git_access_token
arg git_user_name
RUN /bin/bash -c 'git clone https://$git_user_name:$git_access_token@github.com/adhocteam/linkcheck'
RUN /bin/bash -c 'cd linkcheck; GOPATH="/projects/src GOBIN="/projects/bin /usr/local/go/bin/go get .; GOPATH="/projects/src GOBIN="/projects/bin /usr/local/go/bin/go build'
# pull code
RUN /bin/bash -c 'cd linkcheck; fpm -n linkcheck -v 1 -s dir -t rpm -a x86_64 --prefix /bin/ -p linkcheck-latest.rpm linkcheck'
# build app
ENTRYPOINT /bin/bash
