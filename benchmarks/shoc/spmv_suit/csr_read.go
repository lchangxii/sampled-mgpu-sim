package spmv_suit

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include "csr_matrix.cpp"
import "C"
import (
	"fmt"
	"log"
	"unsafe"
	"reflect"
	"gitlab.com/akita/mgpusim/v3/benchmarks/matrix/csr"
)

func carray2slice(array *C.int, len int) []uint32 {
        var list []uint32
        sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&list)))
        sliceHeader.Cap = len
        sliceHeader.Len = len
        sliceHeader.Data = uintptr(unsafe.Pointer(array))
        return list
}
func carray2slice2(array *C.double, len int) []float64 {
        var list []float64
        sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&list)))
        sliceHeader.Cap = len
        sliceHeader.Len = len
        sliceHeader.Data = uintptr(unsafe.Pointer(array))
        return list
}
func arrayD2F( arrayf64 []float64 ) []float32{
    arrayf32 := make( []float32,len(arrayf64) )
    for i, f64 := range arrayf64 {
        arrayf32[i] = float32(f64)
    }
    return arrayf32
}

func get_csr  ( file_location string ) (csr.Matrix,int32,int32) {
    c_file_name := C.CString( (file_location ))
    fmt.Printf("%s\n",c_file_name)
	defer C.free(unsafe.Pointer(c_file_name))
    //sparse_matrix := C.matrix_read_csr( c_file_name )
    sparse_matrix := C.matrix_read_csr( c_file_name )
    row_num := sparse_matrix.row_num
    column_num := sparse_matrix.column_num
    data_num := sparse_matrix.data_num
    log.Printf("collom %d row num %d ",row_num,column_num)
	if row_num != column_num {
        panic( "We now only support square matrix" )
    }

    m := csr.Matrix{}
    m.RowOffsets = carray2slice( sparse_matrix.row_ptr, int(row_num+1))
	m.ColumnNumbers = carray2slice( sparse_matrix.column_ptr,int(data_num ))
    values64 := carray2slice2( sparse_matrix.data_ptr, int(data_num ))
    m.Values = arrayD2F(values64)
//    for _,i32 := range m.RowOffsets {
//        fmt.Printf("%d \n",i32)
//    }
//    fmt.Printf("%d %d %d",len(m.Values),len(m.RowOffsets),len(m.ColumnNumbers))
    //C.csr_spmv( sparse_matrix,  )
    return m,int32(row_num),int32(data_num)

}
