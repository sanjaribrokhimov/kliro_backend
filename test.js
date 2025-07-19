fetch("https://kliro.uz/auth/register", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({email: "test@test.com"})
  }).then(r => r.json()).then(console.log).catch(console.error);