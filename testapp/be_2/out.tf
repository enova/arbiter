output "notes" {
  value = ["do", "re", "mi", "fa", "so", "la", "ti"]
}

output "complex" {
  value = {
    "some": "data",
    "with": ["nested", "types", "and"],
    "numbers": {
      "like": 1,
      "and": 2
    },
    "plus": {
      "boolean": {
        "like": true,
        "and": false
      }
    },
    "not to mention": null
  }
}
