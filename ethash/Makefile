all: cl.go gcn.go

pl.go:
	go-bindata -o pl -pkg cl cl/kernel1.cl

pcn.go:
	go-bindata -o pcn2 	-pkg gcn gcn/

clean:
	rm -rf cl/*.go
	rm -rf gcn/*.go
