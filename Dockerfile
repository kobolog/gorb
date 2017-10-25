FROM golang:1.8

ENV DEBIAN_FRONTEND=noninteractive
RUN  apt-get update \
  && apt-get install -y software-properties-common python-pip \
  python-setuptools \
  python-dev \
  build-essential \
  libssl-dev \
  libffi-dev \
  && apt-get install --no-install-suggests --no-install-recommends -y \
  curl \
  git \
  build-essential \
  python-netaddr \
  unzip \
  vim \
  wget \
  inotify-tools \
  && apt-get clean -y \
  && apt-get autoremove -y \
  && rm -rf /var/lib/apt/lists/* /tmp/*

RUN pip install pyinotify

# install glide to manage dependencies
ENV GLIDEVERSION=0.12.3
RUN wget https://github.com/Masterminds/glide/releases/download/v${GLIDEVERSION}/glide-v${GLIDEVERSION}-linux-amd64.tar.gz
RUN mkdir glide-install ; tar xzf glide-v${GLIDEVERSION}-linux-amd64.tar.gz -C glide-install
RUN mv glide-install/linux-amd64/glide /usr/local/bin/ ; rm -rf glide-install

# Grab the source code and add it to the workspace.
ENV PATHWORK=/go/src/github.com/kobolog/gorb
ADD ./ $PATHWORK
WORKDIR $PATHWORK

RUN glide install -v

ADD ./docker/* /
RUN chmod 755 /entrypoint.sh
RUN chmod 755 /autocompile.py
CMD /entrypoint.sh
