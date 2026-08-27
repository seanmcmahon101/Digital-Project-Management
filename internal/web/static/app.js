// Digital Project Management — client behaviour.
// Kept deliberately small: htmx boosts navigation; this file adds
// responsive navigation, keyboard search, and kanban drag & drop.

(function () {
  'use strict';

  // Auto-dismiss flash messages.
  function armFlash(root) {
    (root || document).querySelectorAll('.flash').forEach(function (el) {
      setTimeout(function () {
        el.style.transition = 'opacity .4s';
        el.style.opacity = '0';
        setTimeout(function () { el.remove(); }, 450);
      }, 5000);
    });
  }

  // Kanban drag & drop: cards carry data-task-id, columns data-status.
  function armKanban(root) {
    var scope = root || document;
    scope.querySelectorAll('.kcard[draggable]').forEach(function (card) {
      card.addEventListener('dragstart', function (e) {
        card.classList.add('dragging');
        e.dataTransfer.setData('text/plain', card.dataset.taskId);
        e.dataTransfer.effectAllowed = 'move';
      });
      card.addEventListener('dragend', function () {
        card.classList.remove('dragging');
      });
    });
    scope.querySelectorAll('.kanban-col').forEach(function (col) {
      col.addEventListener('dragover', function (e) {
        e.preventDefault();
        col.classList.add('drag-over');
      });
      col.addEventListener('dragleave', function () {
        col.classList.remove('drag-over');
      });
      col.addEventListener('drop', function (e) {
        e.preventDefault();
        col.classList.remove('drag-over');
        var taskId = e.dataTransfer.getData('text/plain');
        if (!taskId) return;
        var body = new URLSearchParams();
        body.set('status', col.dataset.status);
        fetch('/tasks/' + taskId + '/status', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: body.toString()
        }).then(function (response) {
          if (!response.ok) throw new Error('Task move failed');
          // Re-render the board region via htmx for a smooth swap.
          if (window.htmx) {
            htmx.ajax('GET', window.location.pathname + window.location.search, {
              target: '#page', select: '#page', swap: 'outerHTML'
            });
          } else {
            window.location.reload();
          }
        }).catch(function () {
          window.location.reload();
        });
      });
    });
  }

  function armNavigation() {
    var toggle = document.getElementById('nav-toggle');
    var scrim = document.getElementById('nav-scrim');
    var sidebar = document.getElementById('primary-sidebar');
    if (!toggle || !scrim || !sidebar || toggle.dataset.armed) return;
    toggle.dataset.armed = 'true';

    function closeNav() {
      document.body.classList.remove('nav-open');
      toggle.setAttribute('aria-expanded', 'false');
    }
    function openNav() {
      document.body.classList.add('nav-open');
      toggle.setAttribute('aria-expanded', 'true');
      var first = sidebar.querySelector('input, a');
      if (first) first.focus();
    }
    toggle.addEventListener('click', function () {
      if (document.body.classList.contains('nav-open')) closeNav();
      else openNav();
    });
    scrim.addEventListener('click', closeNav);
    sidebar.addEventListener('click', function (event) {
      if (event.target.closest('a')) closeNav();
    });
    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape' && document.body.classList.contains('nav-open')) {
        closeNav();
        toggle.focus();
      }
    });
  }

  function armSearchShortcut() {
    if (document.documentElement.dataset.searchShortcutArmed) return;
    document.documentElement.dataset.searchShortcutArmed = 'true';
    document.addEventListener('keydown', function (event) {
      var target = event.target;
      var typing = target && (target.matches('input, textarea, select') || target.isContentEditable);
      var shortcut = event.key === '/' || ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k');
      if (!shortcut || typing || event.altKey) return;
      event.preventDefault();
      var input = document.getElementById('global_search') || document.getElementById('search_page_query');
      if (input) input.focus();
    });
  }

  function armAccessibility(root) {
    var scope = root || document;
    scope.querySelectorAll('a.active').forEach(function (link) {
      link.setAttribute('aria-current', 'page');
    });
    scope.querySelectorAll('form[data-confirm], form[action$="/delete"]').forEach(function (form) {
      if (form.dataset.confirmArmed || form.querySelector('[name="confirm_code"]')) return;
      form.dataset.confirmArmed = 'true';
      form.addEventListener('submit', function (event) {
        var message = form.dataset.confirm || 'Delete this item? This cannot be undone.';
        if (!window.confirm(message)) event.preventDefault();
      });
    });
    scope.querySelectorAll('[data-action="print"]').forEach(function (button) {
      if (button.dataset.actionArmed) return;
      button.dataset.actionArmed = 'true';
      button.addEventListener('click', function () { window.print(); });
    });
    scope.querySelectorAll('select[data-auto-submit]').forEach(function (select) {
      if (select.dataset.autoSubmitArmed) return;
      select.dataset.autoSubmitArmed = 'true';
      select.addEventListener('change', function () { select.form.requestSubmit(); });
    });
  }

  function armAll(root) {
    armFlash(root);
    armKanban(root);
    armNavigation();
    armSearchShortcut();
    armAccessibility(root);
  }

  document.addEventListener('DOMContentLoaded', function () { armAll(document); });
  document.body.addEventListener('htmx:afterSwap', function (e) { armAll(e.target); });
})();
