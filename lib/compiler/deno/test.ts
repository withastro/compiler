import { transform } from "./mod.ts";

;(async () => {
    const res = await transform('<h1>Hello world!</h1>');
    console.log(res)
})()
