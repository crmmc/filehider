# filehider
The FileHider

A simple tool to hide your file

Usage: 
    filehider [Option] [File1] [File2] [File...]

The Program will automatic encode/decode files

Options:
    -n use for disable sha1 check,but it can only be used when decoding file
    -f Remove the "_" before the output file name
    -s Turn on the auto rename switch,the output file name can cheat mechine prefectly
    --ext=[extname] can reset the output file suffix (default: mp4)
Example:
    filehider C:/pagefile.sys D:/test.zip --ext=mov -n -s 
