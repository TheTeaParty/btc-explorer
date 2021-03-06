package data

const TxMapping = `
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "txid": {
        "type": "keyword"
      },
      "fee": {
        "type": "double"
      },
      "blockhash": {
        "type": "keyword"
      },
      "vins": {
        "type": "nested",
        "properties": {
          "address": {
            "type": "keyword"
          },
          "value": {
            "type": "double"
          }
        }
      },
      "vouts": {
        "type": "nested",
        "properties": {
          "address": {
            "type": "keyword"
          },
          "value": {
            "type": "double"
          }
        }
      },
      "time": {
        "type": "long"
      }
    }
  }
}
`

const BlockMapping = `
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "hash": {
        "type": "keyword"
      },
      "strippedsize": {
        "type": "integer"
      },
      "size": {
        "type": "integer"
      },
      "weight": {
        "type": "integer"
      },
      "height": {
        "type": "integer"
      },
      "versionHex": {
        "type": "text"
      },
      "merkleroot": {
        "type": "text"
      },
      "tx": {
        "properties": {
          "txid": {
            "type": "keyword"
          },
          "hash": {
            "type": "keyword"
          },
          "version": {
            "type": "short"
          },
          "size": {
            "type": "integer"
          },
          "vsize": {
            "type": "integer"
          },
          "locktime": {
            "type": "long"
          },
          "vin": {
            "properties": {
              "txid": {
                "type": "keyword"
              },
              "vout": {
                "type": "short"
              },
              "scriptSig": {
                "properties": {
                  "asm": {
                    "type": "text"
                  }
                }
              },
              "sequence": {
                "type": "long"
              },
              "txinwitness": {
                "type": "keyword"
              }
            }
          },
          "vout": {
            "properties": {
              "value": {
                "type": "double"
              },
              "n": {
                "type": "short"
              },
              "scriptPubKey": {
                "properties": {
                  "asm": {
                    "type": "text"
                  },
                  "reqSigs": {
                    "type": "short"
                  },
                  "type": {
                    "type": "text"
                  },
                  "addresses": {
                    "type": "keyword"
                  }
                }
              }
            }
          }
        }
      },
      "time": {
        "type": "long"
      },
      "mediantime": {
        "type": "long"
      },
      "nonce": {
        "type": "long"
      },
      "bits": {
        "type": "text"
      },
      "difficulty": {
        "type": "double"
      },
      "chainwork": {
        "type": "text"
      },
      "previoushash": {
        "type": "keyword"
      },
      "nexthash": {
        "type": "keyword"
      }
    }
  }
}
`

const VoutMapping = `
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "txidbelongto": {
        "type": "keyword"
      },
      "value": {
        "type": "double"
      },
      "voutindex": {
        "type": "keyword"
      },
      "coinbase": {
        "type": "boolean"
      },
      "addresses": {
        "type": "keyword"
      },
      "time": {
        "type": "long"
      },
      "used": {
        "properties": {
          "txid": {
            "type": "keyword"
          },
          "vinindex": {
            "type": "short"
          }
        }
      }
    }
  }
}
`

const BalanceMapping = `
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "address": {
        "type": "keyword"
      },
      "amount": {
        "type": "double"
      }
    }
  }
}
`

const BalanceJournalMapping = `
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "address": {
        "type": "keyword"
      },
      "amount": {
        "type": "double"
      },
      "txid": {
        "type": "keyword"
      },
      "operate": {
        "type": "text"
      }
    }
  }
}
`
