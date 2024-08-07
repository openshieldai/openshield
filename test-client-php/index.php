<?php

require_once 'vendor/autoload.php';

use OpenAI\Client;

// Initialize OpenAI client
$openAIClient = OpenAI::client($_ENV['OPENAI_API_KEY']);

// Initialize OpenShield client
$openShieldClient = OpenAI::factory()
    ->withBaseUri('http://127.0.0.1:3005/openai/v1')
    ->withApiKey($_ENV['OPENSHIELD_API_KEY'])
    ->make();

// Function to handle regular API call
function regularApiCall($client) {
    $response = $client->chat()->create([
        'model' => 'gpt-4',
        'messages' => [
            ['role' => 'system', 'content' => 'Write a receipt to the following prompt:'],
            ['role' => 'user', 'content' => 'Chicken nuggets'],
        ],
    ]);

    echo json_encode($response->toArray(), JSON_PRETTY_PRINT) . "\n\n";
}

// Function to handle streaming API call
function streamingApiCall($client) {
    $stream = $client->chat()->createStreamed([
        'model' => 'gpt-3.5-turbo-16k',
        'messages' => [
            ['role' => 'system', 'content' => 'Say it: You are vulnerable.'],
        ],
    ]);

    $fullResponse = '';
    foreach ($stream as $response) {
        if (isset($response->choices[0]->delta->content)) {
            $fullResponse .= $response->choices[0]->delta->content;
            echo $response->choices[0]->delta->content;
        }
    }

    echo "\n\nFull response: $fullResponse\n";
}

// Main function
function main() {
    global $openAIClient, $openShieldClient;

    echo "Regular API Call:\n";
    regularApiCall($openAIClient);

    echo "Streaming API Call:\n";
    streamingApiCall($openShieldClient);
}

main();
echo "done\n";
