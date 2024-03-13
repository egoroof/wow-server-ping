export function mean(values) {
  if (values.length === 0) {
    return 0;
  }
  values.sort((a, b) => a - b); // todo не мутировать входящий массив
  const meanIndex = Math.floor(values.length / 2);

  if (isOdd(values.length)) {
    return values[meanIndex];
  } else {
    // чётное количество элементов - находим avg от двух посередине
    const leftIndex = meanIndex - 1;
    const left = values[leftIndex];
    const right = values[meanIndex];

    return Math.round((left + right) / 2);
  }
}

export function avg(values) {
  if (values.length === 0) {
    return 0;
  }

  let sum = 0;
  values.forEach((elem) => {
    sum += elem;
  });
  return Math.round(sum / values.length);
}

export function jitter(values) {
  if (values.length === 0) {
    return 0;
  }

  let min = Number.MAX_SAFE_INTEGER;
  let max = 0;
  values.forEach((value) => {
    if (min > value) {
      min = value;
    }
    if (max < value) {
      max = value;
    }
  });

  return max - min;
}

function isOdd(number) {
  return Boolean(number & 1);
}
