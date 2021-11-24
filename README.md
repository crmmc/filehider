# filehider

The FileHider

build:
go build -ldflags="-w -s" // remove debug information for output program
upx --best filehider.exe // use upx to compress the program for reducing its size

A simple tool to hide your file,multithreading support!
Usage: filehider [Option] [File1] [File2] [path1] [path2] [File...] [path...]

Program will automatic encode/decode files, have buildin multithread library support!

Options:
"-n" use for disable sha1 check,but it can only be used when decoding file
"-f" Remove the "\_" before the output file name
"-s" Turn on the auto rename switch,the output file name can cheat mechine prefectly
"--ext=" can reset the output file suffix (default: mp4)
"--outputpath=" can set the path for all output files (defalut:[like the input file])
"--maxthread=[int]" can set the max threads number for filehider,default thread number is your cpu's cores number
"-t" Enable the only test mode, will only test encrypted file data but not write decrypted data to new file

Example:
filehider C:/pagefile.sys D:/test.zip E:/files --ext=mov --outputpath=./output/ -n -s
