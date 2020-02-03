# ggpack

Tool for inspecting and extracting from data files of the game [Thimbleweed Park](https://thimbleweedpark.com/).

I've build it to introspect the data files of the game which I really enjoyed playing.
You should consider to buy it as it is really worth the money.

Inspirations how the decoding has to work are taken from [engge](https://github.com/scemino/engge)
and [NGGPack](https://github.com/scemino/NGGPack).

# Build

You need a recent [Go](https://golang.org) development setup.
Tested successfully with 1.13.7. Earlier and later versions should
work also.

```(shell)
go get github.com/s-l-teichmann/ggpack/cmd/ggpack
```
# Usage

The tool has two modes to operate in. The first (default) is listing.
```(shell)
ggpack /path/to/the/ThimbleweedPark.ggpack1
```
This will simply dump the names of the files stored in the ggpack container
along with there sizes.

The second mode is extracting.
```(shell)
ggpack  --dir bnuts --extract 'bnut$' /path/to/the/ThimbleweedPark.ggpack1
```

This will extract all files from the container with ``bnut`` to the
directory ``bnuts`` (which has to exist). The ``--dir`` option defaults
to the current directory. The ``--extract`` option takes a regular expression
to be matched against the file names.

## License

This is Free and open source software governed by the MIT license.
