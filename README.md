[![Release Workflow](https://github.com/Tensai75/nxg-loader/actions/workflows/build_and_publish.yml/badge.svg?event=release)](https://github.com/Tensai75/nxg-loader/actions/workflows/build_and_publish.yml)
[![Latest Release)](https://img.shields.io/github/v/release/Tensai75/nxg-loader?logo=github)](https://github.com/Tensai75/nxg-loader/releases/latest)

# NxG Loader
Proof of Concept for a new way of binary download from Usenet using the NxG Header, eliminating the need for Usenet search engines and NZB files.

The NxG Loader only works with NxG Headers created by an NxG Header compatible Usenet upload tool, e.g. the [NxG Upper](https://github.com/Tensai75/nxg-upper/).

NxG Loader also accepts the new NXGLNK URI scheme and can register itself as a target for NXGLNKs.

## Advantages of the NxG Header
With the NxG Header, neither Usenet search engines nor NZB files are needed for binary downloads. The message IDs required to retrieve the articles are calculated directly from the NxG Header.
Par2 files are only downloaded if missing or corruptes messages are detected. However, one disadvantage is that in this case all par2 files must currently be downloaded, even if only one block is corrupted.

## Requirements
NxG Loader requires that unrar.exe and par2.exe (par2cmdline) are installed on your system and the paths to the executables are specified in the nxg-loader.conf.
The required executables are freeware and can be downloaded here:

- par2: https://github.com/animetosho/par2cmdline-turbo/releases
- unrar: https://www.rarlab.com/rar_add.htm

## Installation
1. Download the executable file for your system from the release page.
2. Extract the archive to a folder and run the executable.
3. An nxg-loader.conf configuration file is created in this folder (or in "~/.conf/" for Linux systems).
4. Edit the nxg-loader.conf according to your requirements.

## Running the program
Run the program in a cmd line with the following flags:

`nxg-loader --header "[NXGHEADER]" --password "[PASSWORD]" --title "[TITLE]"`

or by specifying an NXGLNK as a positional argument:

`nxg-loader "nxglnk://?h=[NXGHEADER]&p=[PASSWORD]&t=[TITLE]"`

- `[NXGHEADER]` = the NXG Header for this download (required)
- `[PASSWORD]` = password required to extract the download (optional)
- `[TITLE]` = title of the download (optional)

See the other command line arguments and options with:

`nxg-loader -h`

Please also read the nxg-loader.conf for additional explanations in the comments

## Todos
A lot...

This is a Proof of Concept with the minimum necessary features. 
So there is certainly a lot left to do.

## Version history
### beta 1
- first public version

## Credits
This software is built using golang ([License](https://go.dev/LICENSE)).

This software uses the following external libraries:
- github.com/acarl005/stripansi ([License](https://github.com/acarl005/stripansi/blob/master/LICENSE))
- github.com/alexflint/go-arg ([License](https://github.com/alexflint/go-arg/blob/master/LICENSE))
- github.com/chrisfarms/yenc ([License](github.com/chrisfarms/yenc))
- github.com/schollz/progressbar/v3 ([License](https://github.com/schollz/progressbar/blob/main/LICENSE))
- github.com/spf13/viper ([License](https://github.com/spf13/viper/blob/master/LICENSE))
