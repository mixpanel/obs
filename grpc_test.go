package obs

import "fmt"

func Example_formatRPCName() {
	fmt.Println(formatRPCName("/mixpanel.arb.pb.StorageServer/Tail"))
	// Output: StorageServer.Tail
}
