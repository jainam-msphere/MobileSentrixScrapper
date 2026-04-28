to start server
`~/scrapper/backend/cmd$ go run .`

### Current logic

- List of manufacturers and correspnding devices would be fetched from GSMArena.
- When you select appropriate manufacturer and its correspnding device, and click fetch then data from the GSMArena website would be scrapped and provided
- You can also provide phonedb link with above api which would append additional data fetched from phonedb
- You can also provide just phonedb link, just use path params none in "brand_name" & "item_name" in /brands/{brand_name}/devices/{item_name}/sources/{source_type}

### Points to remember

- all apis follow openapi file in config
- GSMArena data has higher priority than phonedb Data
