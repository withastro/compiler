/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  await transform(
    `---
import CartItems from './CartItems.astro';
---

<script>
  document.addEventListener('alpine:init', () => {
    Alpine.data('initCartDrawer', () => ({
      open: false,
      cart: {},
      getData(data) {
          if (data.cart) {
              this.cart = data.cart
              this.setCartItems();
          }
      },
      cartItems: [],
      setCartItems() {
          this.cartItems = this.cart && this.cart.items.sort(function(a,b) { return a.item_id - b.item_id }) || []
      },
      deleteItemFromCart(itemId) {
        var formKey = document.querySelector('input[name=form_key]').value;
        fetch(BASE_URL+"checkout/sidebar/removeItem/", {
            "headers": {
                "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
            },
            "body": "form_key="+ formKey + "&item_id="+itemId,
            "method": "POST",
            "mode": "cors",
            "credentials": "include"
        }).then(function (response) {
            if (response.redirected) {
                window.location.href = response.url;
            } else if (response.ok) {
                return response.json();
            } else {
                typeof window.dispatchMessages !== "undefined" && window.dispatchMessages(
                    [{
                        type: "warning",
                        text: "Could not remove item from quote."
                    }], 5000
                );
            }
        }).then(function (response) {
            typeof window.dispatchMessages !== "undefined" && window.dispatchMessages(
                [{
                    type: response.success ? "success" : "error",
                    text: response.success
                        ? "You removed the item."
                        : response.error_message
                }], 5000
            );
            var reloadCustomerDataEvent = new CustomEvent("reload-customer-section-data");
            window.dispatchEvent(reloadCustomerDataEvent);
        });
      }
    }))
  })
</script>

<section id="cart-drawer"
         x-data="initCartDrawer"
         @private-content-loaded.window="getData(event.detail.data)"
         @toggle-cart.window="open=true;"
         @keydown.window.escape="open=false"
>
  <template x-if="cart && cart.summary_count">
    <div role="dialog"
             aria-labelledby="cart-drawer-title"
             aria-modal="true"
             @click.outside="open = false"
             class="fixed inset-y-0 right-0 z-30 flex max-w-full">
      <div class="backdrop"
        x-show="open"
        x-transition:enter="ease-in-out duration-500"
        x-transition:enter-start="opacity-0"
        x-transition:enter-end="opacity-100"
        x-transition:leave="ease-in-out duration-500"
        x-transition:leave-start="opacity-100"
        x-transition:leave-end="opacity-0"
        @click="open = false"
        aria-label="Close panel">
      </div>
      <div class="relative w-screen max-w-md shadow-2xl"
        x-show="open"
        x-transition:enter="transform transition ease-in-out duration-500 sm:duration-700"
        x-transition:enter-start="translate-x-full"
        x-transition:enter-end="translate-x-0"
        x-transition:leave="transform transition ease-in-out duration-500 sm:duration-700"
        x-transition:leave-start="translate-x-0"
        x-transition:leave-end="translate-x-full"
      >
        <div
          x-show="open"
          x-transition:enter="ease-in-out duration-500"
          x-transition:enter-start="opacity-0"
          x-transition:enter-end="opacity-100"
          x-transition:leave="ease-in-out duration-500"
          x-transition:leave-start="opacity-100"
          x-transition:leave-end="opacity-0" class="absolute top-0 right-0 flex p-2 mt-2"
        >
          <button @click="open = false;" aria-label="Close panel"
                  class="p-2 text-gray-300 transition duration-150 ease-in-out hover:text-black">
            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round"
                    stroke-width="2" d="M6 18L18 6M6 6l12 12">
              </path>
            </svg>
          </button>
        </div>
        <div class="flex flex-col h-full py-6 space-y-6 bg-white shadow-xl">
          <header class="px-4 sm:px-6">
              <h2 id="cart-drawer-title" class="text-lg font-medium leading-7 text-gray-900">My Cart</h2>
          </header>
        <div class="relative grid gap-6 px-4 py-6 overflow-y-auto bg-white border-b
            sm:gap-8 sm:px-6 border-container">
          <template x-for="item in cartItems">


            <!-- <CartItems/> -->
<div class="flex items-start p-3 -m-3 space-x-4 transition duration-150
    ease-in-out rounded-lg hover:bg-gray-100">
  <a :href="item.product_url" class="w-1/4">
    <img
        :src="item.product_image.src"
        :width="item.product_image.width"
        :height="item.product_image.height"
        loading="lazy"
    />
  </a>
  <div class="w-3/4 space-y-2">
    <div>
      <p class="text-xl">
          <span x-html="item.qty"></span> x <span x-html="item.product_name"></span>
      </p>
      <p class="text-sm"><span x-html="item.product_sku"/></p>
    </div>
    <template x-for="option in item.options">
        <div class="pt-2">
            <p class="font-semibold" x-text="option.label + ':'"></p>
            <p class="text-secondary" x-html="option.value"></p>
        </div>
    </template>
    <p><span x-html="item.product_price"></span></p>
    <div class="pt-4">
      <a :href="item.configure_url"
          x-show="item.product_type !== 'grouped'"
          class="inline-flex p-2 mr-2 btn btn-primary">
          <svg fill="none" viewBox="0 0 24 24" stroke="currentColor"
                size="16" class="w-5 h-5">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536
                    3.536L6.5 21.036H3v-3.572L16.732 3.732z">
              </path>
          </svg>
      </a>
    </div>
  </div>
</div>



          </template>
        </div>
        <div class="relative grid gap-6 px-4 py-6 bg-white sm:gap-8 sm:px-6">
          <div class="w-full p-3 -m-3 space-x-4 transition duration-150 ease-in-out rounded-lg
              hover:bg-gray-100">
              <p>Subtotal: <span x-html="cart.subtotal"></span></p>
          </div>
          <div class="w-full p-3 -m-3 space-x-4 transition duration-150 ease-in-out rounded-lg hover:bg-gray-100">
            <a @click.prevent.stop="$dispatch('toggle-authentication',
                {url: 'checkout'});"
                href="checkout"
                class="inline-flex btn btn-primary">
                Checkout
            </a>
            <span>or</span>
            <a href="checkout/cart"
                class="underline">
                View and Edit Cart
            </a>
          </div>
        </div>
      </div>
    </div>
  </template>
</section>`,
    {
      sourcemap: true,
    }
  );
}

const MAX_CONCURRENT_RENDERS = 25;
const MAX_RENDERS = 1e4;

async function test() {
  await run();
  const promises = [];
  let tests = [];

  for (let i = 0; i < MAX_RENDERS; i++) {
    tests.push(() => {
      if (i % 1000 === 0) {
        console.log(`Test ${i}`);
      }
      return run();
    });
  }

  // Throttle the paths to avoid overloading the CPU with too many tasks.
  for (const ts of throttle(MAX_CONCURRENT_RENDERS, tests)) {
    for (const t of ts) {
      promises.push(t());
    }
    // This blocks generating more paths until these 10 complete.
    await Promise.all(promises);
    // This empties the array without allocating a new one.
    promises.length = 0;
  }
}

// Throttle the rendering a paths to prevents creating too many Promises on the microtask queue.
function* throttle(max, tests) {
  let tmp = [];
  let i = 0;
  for (let t of tests) {
    tmp.push(t);
    if (i === max) {
      yield tmp;
      // Empties the array, to avoid allocating a new one.
      tmp.length = 0;
      i = 0;
    } else {
      i++;
    }
  }

  // If tmp has items in it, that means there were less than {max} paths remaining
  // at the end, so we need to yield these too.
  if (tmp.length) {
    yield tmp;
  }
}

test();
