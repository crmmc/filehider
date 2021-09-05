package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"
)

var disablesha1 bool           //SHA1校验开关
var fileextname string = "mp4" //输出的加密文件的后缀名
var cfilename string = "_"     //还原的文件的文件名前缀，防止源文件存在导致覆盖写入
var buffersize int = 8192      //读入缓存空间大小
var fileheadle []byte = []byte{
	0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x6D, 0x70, 0x34, 0x32, 0x00, 0x00, 0x00, 0x00,
	0x6D, 0x70, 0x34, 0x32, 0x6D, 0x70, 0x34, 0x31, 0x00, 0x04, 0x38, 0xDC, 0x6D, 0x6F, 0x6F, 0x76,
	0x00, 0x00, 0x00, 0x6C, 0x6D, 0x76, 0x68, 0x64, 0x00, 0x00, 0x00, 0x00, 0xDD, 0x53, 0x54, 0x50,
	0xDD, 0x53, 0x54, 0x5C, 0x00, 0x01, 0x5F, 0x90, 0x01, 0xD5, 0x94, 0x3F, 0xDD, 0x53} //用于校验和伪装的文件头

func main() {
	argv := os.Args   //系统传入的命令行参数列表
	argc := len(argv) //系统传入的命令行参数个数
	if argc == 3 {
		if argv[2] == "-n" {
			disablesha1 = true //-n开关被开启
		}
	} else if argc != 2 {
		help(argv[0])
		os.Exit(1)
	}
	inf, inferr := os.Open(argv[1]) //打开输入文件
	if inferr != nil {
		fmt.Println("Open Input File " + argv[1] + " Failed!")
	}
	var outfn string = ""                 //输出文件
	orfp, orfn := filepath.Split(argv[1]) //分割输入文件的路径和文件名
	//读识别用文件头
	readsize1 := len(fileheadle)
	infheadle := make([]byte, readsize1)
	inf.Read(infheadle)
	//
	sha := sha1.New() //sha1检验对象
	//分析是否符合文件头
	if analyze(fileheadle, infheadle) {
		//符合文件头，判断为已被加密的文件
		fmt.Println("Mode: Decode")
		if disablesha1 {
			fmt.Println("SHA1 Check: Disable")
		} else {
			fmt.Println("SHA1 Check: Enable")
		}
		if orfp == "" {
			orfp = "./" //防止出现空文件路径
		}
		// 读储存的文件名变量大小
		nbl := make([]byte, 4)
		inf.Read(nbl)
		nl := BytesToInt(nbl) //转换为int
		// 读储存的文件名
		rbn := make([]byte, nl)
		inf.Read(rbn)
		rn := bytes2str(encode(rbn)) //转换为字符串
		rn = cfilename + rn
		outfn = orfp + rn //组合输出文件路径
		outf, oerr := os.Create(outfn)
		if oerr != nil {
			fmt.Println("Open Output File " + outfn + " Failed")
			os.Exit(4)
		}
		if disablesha1 {
			fmt.Println("SHA1 Check: Disable")
		}
		rbshadata := make([]byte, 20) //储存从文件读到的sha1
		inf.Read(rbshadata)           //读sha1值
		buf := make([]byte, buffersize)
		for {
			count, rerr := inf.Read(buf)
			if rerr == io.EOF {
				break
			}
			debuf := encode(buf[0:count]) //文件内容解密
			if !disablesha1 {
				sha.Write(debuf) //文件内容读入sha1
			}
			outf.Write(debuf) //写入还原的数据到输出文件
		}
		outf.Close() //写入完成，关闭输出文件
		if !disablesha1 {
			rshadata := sha.Sum(nil) //计算sha1
			if analyze(rshadata, rbshadata) {
				fmt.Println("SHA1 Check: Okey")
			} else {
				fmt.Println("SHA1 Check: File may broken")
			}
		} else {
			fmt.Println("SHA1 Check: Please check file sha1 by yourself")
		}
	} else {
		inf.Seek(0, io.SeekStart)           //是个未加密的文件，移动文件指针到文件开头
		outfn = argv[1] + "." + fileextname //生成文件名
		fmt.Println("Mode: Encode")
		fmt.Println("SHA1 Check: Enable")
		if disablesha1 {
			fmt.Println("SHA1 Check: ignore \"-b\" switch")
		}
		outf, oerr := os.Create(outfn) //若文件存在则覆盖写入，不存在就创建
		if oerr != nil {
			fmt.Println("Open Output File " + outfn + " Failed")
			os.Exit(2)
		}
		outf.Write(fileheadle) //写文件头
		//转换并写入文件名数据长度和文件名数据
		a1 := str2bytes(orfn)
		outf.Write(IntToBytes(len(a1)))
		outf.Write(encode(a1))
		wshaseek, _ := outf.Seek(0, io.SeekCurrent) //用于储存sha1写入位置的文件指针
		outf.Write(make([]byte, 20))                //先写个空值占位
		buf := make([]byte, buffersize)             //读缓存
		for {
			count, err := inf.Read(buf)
			if err == io.EOF {
				break
			}
			wrb := buf[0:count]
			sha.Write(wrb)
			outf.Write(encode(wrb))
		}
		shadata := sha.Sum(nil)
		outf.Seek(wshaseek, io.SeekStart)     //回到sha1数据的位置
		shatext := fmt.Sprintf("%x", shadata) //sha1数据转换成字符串好展示
		fmt.Println("SHA1 Check: " + shatext)
		outf.Write(shadata) //写sha1值到文件
		outf.Close()
	}
	inf.Close()
	fmt.Println("Done")
}

//帮助
func help(argv0 string) {
	fmt.Println("Usage: " + argv0 + " [FilePath] [Option]\nOption \"-n\" can only be used when decoding file")
}

//二进制数据逐字节比较
func analyze(bigbyte, smallbyte []byte) bool {
	for i := 0; i < len(bigbyte); i++ {
		if bigbyte[i] != smallbyte[i] {
			return false
		}
	}
	return true
}

//整形转换成字节
func IntToBytes(n int) []byte {
	var v2 uint32 = uint32(n)
	var b2 []byte = make([]byte, 4)
	b2[3] = uint8(v2)
	b2[2] = uint8(v2 >> 8)
	b2[1] = uint8(v2 >> 16)
	b2[0] = uint8(v2 >> 24)
	return b2
}

//字节转换成整形
func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)
	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return int(x)
}

//简单的取反加密函数
func encode(inb []byte) []byte {
	for i := 0; i < len(inb); i++ {
		inb[i] = ^inb[i]
	}
	return inb
}

//高效的字符串转字节组
func str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

//高效的字节组转字符串
func bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
