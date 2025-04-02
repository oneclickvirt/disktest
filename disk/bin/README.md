```
wget https://github.com/axboe/fio/archive/refs/tags/fio-3.39.zip
unzip fio-3.39.zip
cd fio-fio-3.39
sudo apt update
sudo apt install -y build-essential pkg-config zlib1g-dev libaio-dev libnuma-dev libssl-dev
./configure
make clean
make CC=gcc CFLAGS="-static" LDFLAGS="-static" FIO_STATIC=1
```

https://github.com/axboe/fio