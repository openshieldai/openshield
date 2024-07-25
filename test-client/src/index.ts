import OpenAI from 'openai';

const client = new OpenAI({
    baseURL: 'http://127.0.0.1:3005/openai/v1',
    apiKey: process.env.OPENSHIELD_API_KEY,
});

async function main() {
    const completion = await client.chat.completions.create({
        model: 'gpt-4',
        messages: [
            {role: 'system', content: 'You are a helpful assistant.'},
            {role: 'user', content: 'What is the meaning of life?'}
        ]
    })
    console.log(JSON.stringify(completion, null, 2));
}

main().then(() => console.log('done'));
