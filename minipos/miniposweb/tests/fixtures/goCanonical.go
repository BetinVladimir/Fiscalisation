package main
import("encoding/json";"os")
func main(){var values []any;d:=json.NewDecoder(os.Stdin);d.UseNumber();if err:=d.Decode(&values);err!=nil{panic(err)};for _,v:=range values{b,e:=json.Marshal(v);if e!=nil{panic(e)};os.Stdout.Write(b);os.Stdout.Write([]byte("\n"))}}
