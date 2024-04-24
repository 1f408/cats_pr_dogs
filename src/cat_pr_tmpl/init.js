document.addEventListener('DOMContentLoaded', function(){
  let ui_div = document.createElement("div");

  let url_btn = document.createElement("button");
  url_btn.setAttribute("class", "copy_url");
  url_btn.innerHTML = "Copy URL";
  url_btn.addEventListener('click', function (){
    navigator.clipboard.writeText(location.href);
  });
  ui_div.appendChild(url_btn);

  ui_div.appendChild(document.createTextNode(" "));

  let auth_id = "auth_account";

  let auth_label = document.createElement("label");
  auth_label.textContent = "Account:";
  auth_label.setAttribute("for", auth_id);
  ui_div.appendChild(auth_label);

  let auth_input = document.createElement("input");
  auth_input.setAttribute("id", auth_id);
  auth_input.setAttribute("class", "auth");
  auth_input.setAttribute("type", "text");
  auth_input.setAttribute("name", "account");
  auth_input.setAttribute("placeholder", "username");
  ui_div.appendChild(auth_input);

  auth_input.addEventListener('change', function (){
    set_auth_account(this.value);
    location.reload();
  });

  get_auth_account().then(function(name) {
    auth_input.setAttribute("value", name);
  });

  document.body.insertBefore(ui_div, document.body.firstChild);
});
