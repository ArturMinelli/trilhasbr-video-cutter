import type { InputHTMLAttributes } from 'react'

import { Input } from './Input'

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>

export const DateTimeInput = (props: Props) => {
  return <Input {...props} type="datetime-local" step={1} />
}
