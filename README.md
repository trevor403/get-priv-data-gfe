# get-priv-data-gfe
Retrieve NvFBC Private Data (UUID) from a GeForce Experience installation

## Method for finding the UUID constant

The C code might look something like this
```
NvFBCCreateParams createParams;
memset(&createParams, 0, sizeof(createParams));

((&createParams.pPrivateData) + 0x0) = 0x00000000;
((&createParams.pPrivateData) + 0x4) = 0x00000000;
((&createParams.pPrivateData) + 0x8) = 0x00000000;
((&createParams.pPrivateData) + 0x10) = 0x00000000;

createParams.dwPrivateDataSize = 16;
```

So we want to find an assignment (MOV dword ptr) of value 16 (10h)

proceeded by 4x evenly spaces 4 byte assignments.

## Usage

There are releases precompiled for you! 
You can find them in the [Releases tab](https://github.com/trevor403/get-priv-data-gfe/releases)

You can get the executable via `go get` as well
```
go get github.com/trevor403/get-priv-data-gfe/cmd/...
```

## Disclaimer
Executing this program may put you in violation of NVIDIA's EULA

I do not provide any legal guarantees around this software or it's usage. However it is my opinion that the Reverse Engineering effort that went into developing it is covered by the DMCA as it promotes interoperability.
