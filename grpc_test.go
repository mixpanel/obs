package obs

import "fmt"

func Example_formatRPCName() {
	fmt.Println(formatRPCName("/Company.Service/Method"))
	// Output: Service.Method
}
