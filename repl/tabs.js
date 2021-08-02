const tabs = {
    '': document.querySelector('a[href="/#/"]'),
    'html': document.querySelector('a[href="/#/html"]'),
    'js': document.querySelector('a[href="/#/js"]')
}
const targets = {
    '': document.querySelector('#output'),
    'html': document.querySelector('#html').parentElement,
    'js': document.querySelector('#js').parentElement
}

window.addEventListener('hashchange', () => {
    const hash = location.hash;
    const target = hash.slice(2);
    for (const [name, el] of Object.entries(targets)) {
        if (name === target) {
            el.classList.add('active');
        } else {
            el.classList.remove('active');
        }
    }
    for (const [name, el] of Object.entries(tabs)) {
        if (name === target) {
            el.classList.add('active');
        } else {
            el.classList.remove('active');
        }
    }
})
