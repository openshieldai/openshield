#!/bin/bash

# Create config file with the OPENAI_API_KEY from environment
cat > /app/conf/server.conf << EOL
[main]
use_cache = true
cache_max = 500

[embedding]
model = openai
openai_key = ${OPENAI_API_KEY}

[vectordb]
collection = data-openai
db_dir = /app/data/vdb
n_results = 5

[auto_update]
enabled = true
threshold = 3

[scanners]
input_scanners = transformer,vectordb,sentiment,yara
output_scanners = similarity,sentiment

[scanner:yara]
rules_dir = /app/data/yara

[scanner:vectordb]
threshold = 0.4

[scanner:transformer]
model = deepset/deberta-v3-base-injection
threshold = 0.98

[scanner:similarity]
threshold = 0.4

[scanner:sentiment]
threshold = 0.7
EOL

echo "Generated server.conf with OPENAI_API_KEY"