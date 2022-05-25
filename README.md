# Scraper

Interface for combining multiple address scrapers into a single application. The invididual scrapers are defined as plugin under the `plugins` directory.

# Build

To build this project simply run the `make build` command from the root directory.

# Run

To run this project, call the `make run` command from the root directory.

# Result

## Spravkaru Scraper

This scraper plugin generates a dedicated `json` file for each individual city in the form of:

```json
{
    "city_name_russian" : "string",
    "city_name_english" : "string",
    "contacts" [
        {
            "zip_code" : "string",
            "english" : {
                "first_name" : "string",
                "last_name" : "string",
                "address" : "string"
            },
            "russian" : {
                "first_name" : "string",
                "last_name" : "string",
                "address" : "string"
            }
        },
        "..."
    ]
}
```
