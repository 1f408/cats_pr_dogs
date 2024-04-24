document.addEventListener('DOMContentLoaded', function(){
  let dialog  = document.createElement("dialog");
  dialog.setAttribute("style", "positon: absolute; top: 0; bottom: 0; left: 0; right: 0; margin: auto;");
  document.body.insertBefore(dialog, document.body.firstChild);

  let label = document.createElement("h5");
  label.textContent = "Label";
  dialog.appendChild(label);

  let text = document.createElement("p");
  dialog.appendChild(text);

  let button = document.createElement("button");
  button.textContent = "OK";
  dialog.appendChild(button);

  button.addEventListener('click', function (){
    dialog.close();
  });

  function open_url_dlg(ev){
    ev.preventDefault();
    let url = this.getAttributeNS('http://www.w3.org/1999/xlink', 'href');
    if (url == null) {
      url = this.href;
    }

    let lbl_msg = "URLが正しいか確認してください！";
    label.textContent = lbl_msg;

    if (url != null) {
      text.textContent = decodeURI(url);
      dialog.showModal(this);
    }
  }

  function open_abs_dlg(ev){
    ev.preventDefault();
    let url = this.getAttributeNS('http://www.w3.org/1999/xlink', 'href');
    if (url == null) {
      url = this.href;
    }
    url = new URL(url);

    let lbl_msg = "パスが正しいか確認してください！";
    label.textContent = lbl_msg;

    if (url != null) {
      let abs = url.pathname + url.search + url.hash;
      text.textContent = decodeURI(abs);
      dialog.showModal(this);
    }
  }

  document.body.querySelectorAll("body > .contents a").forEach(function(el) {
    if(el.href == ""){ return; } 
    let l = new URL(el.href);

    if(window.location.origin != l.origin){
      el.addEventListener("click", open_url_dlg);
    } else if(! l.pathname.startsWith("/file/")){
      el.addEventListener("click", open_abs_dlg);
    }
  });
});
