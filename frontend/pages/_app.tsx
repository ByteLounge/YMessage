import type { AppProps } from 'next/app'
import '@/styles/globals.css'
import Head from 'next/head'

export default function App({ Component, pageProps }: AppProps) {
  return (
    <>
      <Head>
        <title>YMessage - Private. Fast. Beautiful.</title>
        <meta name="description" content="A modern, end-to-end encrypted private messaging platform." />
        <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1" />
        <link rel="icon" href="/favicon.ico" />
      </Head>
      <div className="h-screen w-screen overflow-hidden bg-zinc-950 text-zinc-100 selection:bg-imessage-blue selection:text-white">
        <Component {...pageProps} />
      </div>
    </>
  )
}
