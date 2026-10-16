terraform {
  required_providers {
    xyops = {
      source = "santos-edu/xyops"
    }
  }
}

provider "xyops" {
  # endpoint is required: set it here or provide XYOPS_ENDPOINT.
  endpoint = "https://xyops.example.com"
  # api_key is read from XYOPS_API_KEY by default.
}
