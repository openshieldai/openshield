#!/bin/bash

# Generate the configuration at runtime using the current environment variables
echo "Generating configuration with OPENAI_API_KEY..."
/app/generate_config.sh



echo "Loading datasets ..."
python loader.py --config /app/conf/server.conf --dataset deadbits/vigil-instruction-bypass-ada-002
python loader.py --config /app/conf/server.conf --dataset deadbits/vigil-jailbreak-ada-002
echo " "
echo "Starting API server ..."
cd /app
python vigil-server.py --conf conf/server.conf

