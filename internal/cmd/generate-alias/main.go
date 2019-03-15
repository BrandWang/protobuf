// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run . -execute

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	gengo "github.com/golang/protobuf/v2/cmd/protoc-gen-go/internal_gengo"
	"github.com/golang/protobuf/v2/protogen"
	"github.com/golang/protobuf/v2/reflect/protodesc"
	"github.com/golang/protobuf/v2/reflect/protoreflect"

	descriptorpb "github.com/golang/protobuf/v2/types/descriptor"
	knownpb "github.com/golang/protobuf/v2/types/known"
	pluginpb "github.com/golang/protobuf/v2/types/plugin"
)

func main() {
	run := flag.Bool("execute", false, "Write generated files to destination.")
	flag.Parse()

	// Set of generated proto packages to forward to v2.
	files := []struct {
		goPkg  string
		pbDesc protoreflect.FileDescriptor
	}{{
		goPkg:  "github.com/golang/protobuf/protoc-gen-go/descriptor;descriptor",
		pbDesc: descriptorpb.File_google_protobuf_descriptor_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/protoc-gen-go/plugin;plugin_go",
		pbDesc: pluginpb.File_google_protobuf_compiler_plugin_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/any;any",
		pbDesc: knownpb.File_google_protobuf_any_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/duration;duration",
		pbDesc: knownpb.File_google_protobuf_duration_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/timestamp;timestamp",
		pbDesc: knownpb.File_google_protobuf_timestamp_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/wrappers;wrappers",
		pbDesc: knownpb.File_google_protobuf_wrappers_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/struct;structpb",
		pbDesc: knownpb.File_google_protobuf_struct_proto,
	}, {
		goPkg:  "github.com/golang/protobuf/ptypes/empty;empty",
		pbDesc: knownpb.File_google_protobuf_empty_proto,
	}}

	// For each package, construct a proto file that public imports the package.
	var req pluginpb.CodeGeneratorRequest
	for _, file := range files {
		pkgPath := file.goPkg[:strings.IndexByte(file.goPkg, ';')]
		fd := &descriptorpb.FileDescriptorProto{
			Name:             proto.String(pkgPath + "/" + path.Base(pkgPath) + ".proto"),
			Syntax:           proto.String(file.pbDesc.Syntax().String()),
			Dependency:       []string{file.pbDesc.Path()},
			PublicDependency: []int32{0},
			Options:          &descriptorpb.FileOptions{GoPackage: proto.String(file.goPkg)},
		}
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(file.pbDesc), fd)
		req.FileToGenerate = append(req.FileToGenerate, fd.GetName())
	}

	// Use the internal logic of protoc-gen-go to generate the files.
	gen, err := protogen.New(&req, nil)
	check(err)
	for _, file := range gen.Files {
		if file.Generate {
			gengo.GenerateFile(gen, file)
		}
	}

	// Write the generated files.
	resp := gen.Response()
	if resp.Error != nil {
		panic("gengo error: " + resp.GetError())
	}
	for _, file := range resp.File {
		relPath, err := filepath.Rel(filepath.FromSlash("github.com/golang/protobuf"), file.GetName())
		check(err)

		check(ioutil.WriteFile(relPath+".bak", []byte(file.GetContent()), 0664))
		if *run {
			fmt.Println("#", relPath)
			check(os.Rename(relPath+".bak", relPath))
		} else {
			cmd := exec.Command("diff", relPath, relPath+".bak", "-N", "-u")
			cmd.Stdout = os.Stdout
			cmd.Run()
			os.Remove(relPath + ".bak") // best-effort delete
		}
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
