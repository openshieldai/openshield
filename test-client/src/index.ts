import OpenAI from 'openai';
import { HttpsProxyAgent } from 'https-proxy-agent';

const client = new OpenAI({
    // apiKey: '12345',
    baseURL: 'http://127.0.0.1:3005/openai/v1',
    apiKey: process.env.TEST_API_KEY,
});

async function main() {
    // const model = await client.models.retrieve('gpt-4');
    // console.log(model);
    // const customModel = await client.models.retrieve('davinci:ft-persio-2023-03-06-11-44-36');
    // console.log(customModel);
    // const list = await client.models.list();
    //
    // for await (const model of list) {
    //     console.log(model);
    // }

    const completion = await client.chat.completions.create({
        model: 'gpt-4',
        messages: [
            {role: 'system', content: 'You are a helpful assistant.'},
            {role: 'user', content: 'What is the meaning of life?'}
        ]
    })
    console.log(JSON.stringify(completion, null, 2));
}

main().then(r => console.log('done'));
