document.addEventListener('DOMContentLoaded', function(){
  let ui_div = document.createElement("div");
  ui_div.setAttribute("class", "app_ui");

  let url_btn = document.createElement("button");
  url_btn.innerHTML = "Copy URL";
  url_btn.addEventListener('click', function (){
    navigator.clipboard.writeText(location.href);
  });
  ui_div.appendChild(url_btn);

  document.body.insertBefore(ui_div, document.body.firstChild);
});
