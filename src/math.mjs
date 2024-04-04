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

  return Math.round((max - min) / 2);
}
