#!/bin/bash

# Test script for the chat endpoint
# Make sure to set your OpenAI API key in the .env file first

echo "Testing the chat endpoint..."

# Test data - conversation between user and assistant
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "row_key": "user_1",
        "message": "Hello, I am looking for products in your store. Can you help me find some electronics?"
      },
      {
        "row_key": "assistant_1", 
        "message": "Hello! I would be happy to help you find electronics in our store. Let me search through our product catalog for you."
      },
      {
        "row_key": "user_2",
        "message": "What products do you have available? I am particularly interested in items under $100."
      }
    ]
  }'

echo -e "\n\nTest completed!"
