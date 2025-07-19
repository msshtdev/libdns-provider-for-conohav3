# ConoHa VPS Ver 3.0 for `libdns`

This package implements the [libdns interface](https://github.com/libdns/libdns) for [ConoHa VPS Ver.3.0](https://doc.conoha.jp/products/vps-v3/) using [ConoHa VPS Ver.3.0 Public APIs](https://doc.conoha.jp/reference/api-vps3/).

## Authenticating

The `conohav3` package authenticates using the credentials required by ConoHa's Identity API.

You must provide the following variables:

- **APITenantID**: Your ConoHa **Tenant ID** . This identifies your account's tenant.
- **APIUserID**: Your **User ID** associated with the API credentials.
- **APIPassword**: The **User Password** for the user.
- **Region** *(optional)*: The ConoHa service region. If omitted, defaults to `"c3j1"`.

These credentials are used to obtain a token from the Identity service, which is then used to authorize DNS API requests.

See [Identity APIs](https://doc.conoha.jp/reference/api-vps3/api-identity-vps3/identity-post_tokens-v3/) for more details.


## Example Configuration

```go
p := conohav3.Provider{
    APITenantID: "apiTenantID",
    APIUserID: "apiUserID",
    APIPassword: "apiPassword",
    Region: "region", // optional default value is "c3j1"
}
zone := `example.localhost`

// List existing records
fmt.Printf("List existing records\n")
currentRecords, err := conohav3.GetRecords(context.TODO(), zone)
if err != nil {
    fmt.Printf("%v\n", err)
	return
}
for _, record := range currentRecords {
	fmt.Printf("Exists: %v\n", record)
}
```
