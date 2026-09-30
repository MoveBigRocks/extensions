'use strict';
(() => {
  const root = document.querySelector('main');
  const workspace = root.dataset.workspace;
  const targets = document.getElementById('targets');
  const incidents = document.getElementById('incidents');
  const error = document.getElementById('error');
  const select = document.getElementById('environment');
  const more = document.getElementById('more');
  const refresh = document.getElementById('refresh');
  let next = '', generation = 0, controller;
  function element(tag, text, className) { const node = document.createElement(tag); node.textContent = text; if (className) node.className = className; return node; }
  async function read(path, signal) {
    const join = path.includes('?') ? '&' : '?';
    const response = await fetch('/extensions/operational-health/api/' + path + join + 'workspace=' + encodeURIComponent(workspace), {credentials: 'same-origin', redirect: 'error', signal});
    if (!response.ok) throw new Error(response.status === 401 || response.status === 403 ? 'Your session cannot read this workspace. Sign in with operational access and retry.' : 'Current evidence is unavailable. Retry to refresh it.');
    const data = await response.json();
    if (data.workspaceId !== workspace || data.contract !== 'operational-health/v1') throw new Error('The server returned incompatible workspace evidence.');
    return data;
  }
  function renderTargets(items) {
    targets.replaceChildren();
    for (const item of items.filter(t => !select.value || t.environment === select.value)) {
      const box = element('section', '', 'target');
      box.append(element('p', item.environment + (item.canary ? ' · acceptance canary' : ''), 'label'), element('h3', item.component), element('strong', item.state.toUpperCase(), item.state));
      box.append(element('p', item.observation ? 'Observed ' + new Date(item.observation.observedAt).toLocaleString() : 'No current observation'), element('small', 'Alert delivery: ' + item.deliveryState + ' · Pending evidence: ' + item.pendingProjections));
      if (item.maintenanceUntil) box.append(element('p', 'Deployment window ends ' + new Date(item.maintenanceUntil).toLocaleTimeString()));
      if (item.deploymentState === 'failed' || item.deploymentState === 'expired') box.append(element('p', 'Deployment '+item.deploymentRunId+' '+item.deploymentState+'; operator review required.', 'unhealthy'));
      if (item.ownerId) box.append(element('small','Response owner: '+item.ownerId));
      if (item.observation) for (const check of item.observation.checks) box.append(element('p',check.name+': '+check.state,check.state));
      targets.append(box);
    }
    if (!targets.children.length) targets.append(element('p', 'No monitoring sources are configured for this selection.', 'empty'));
  }
  function renderIncidents(page, append) {
    if (!append) incidents.replaceChildren();
    for (const incident of page.items) {
      const row = element('article', '');
      row.append(element('h3', incident.component + ' · ' + incident.environment), element('p', 'Signal: ' + incident.signalState + ' · Case: ' + (incident.caseStatus || 'being created') + ' · Severity: ' + incident.severity));
      row.append(element('small', 'Updated ' + new Date(incident.updatedAt).toLocaleString() + (incident.canary ? ' · Acceptance canary' : '')));
      if (incident.caseId) { const link = element('a', 'Open incident case', 'case'); link.href = '/cases/' + encodeURIComponent(incident.caseId) + '?workspace=' + encodeURIComponent(workspace); row.append(document.createElement('br'), link); }
      else row.append(element('p', 'Case delivery is pending. Evidence is retained and will be retried.'));
      const evidenceButton=element('button','Inspect signal evidence');evidenceButton.type='button';
      const detail=element('section','');detail.hidden=true;
      evidenceButton.addEventListener('click',async()=>{
        if (!detail.hidden){detail.hidden=true;evidenceButton.textContent='Inspect signal evidence';return;}
        detail.hidden=false;evidenceButton.textContent='Hide signal evidence';await loadEvidence(incident.id,detail);
      });row.append(document.createElement('br'),evidenceButton,detail);
      incidents.append(row);
    }
    if (!incidents.children.length) incidents.append(element('p', 'No incidents match this selection. Current health is shown above.', 'empty'));
    next = page.next || ''; more.hidden = !next;
  }

  async function loadEvidence(id,container,signalsAfter='',projectionsAfter='') {
    container.replaceChildren(element('p','Loading evidence…'));
    const abort=new AbortController();const timer=setTimeout(()=>abort.abort(),15000);
    try {
      const query=new URLSearchParams({limit:'20',signalsAfter,projectionsAfter});
      const data=await read('incidents/'+encodeURIComponent(id)+'?'+query,abort.signal);
      container.replaceChildren(element('h4','Signal history'));
      for (const signal of data.evidence.signals.items) container.append(element('p',signal.alertName+' · '+signal.status+' · started '+new Date(signal.startsAt).toLocaleString()+' · received '+new Date(signal.receivedAt).toLocaleString()));
      container.append(element('h4','Case delivery'));
      for (const projection of data.evidence.projections.items){
        container.append(element('p',projection.kind+' · '+projection.state+' · attempts '+projection.attempts+(projection.lastError?' · '+projection.lastError:'')));
        if(projection.state==='quarantined') {
          const retry=element('button','Retry case delivery');retry.type='button';
          retry.addEventListener('click',async()=>{
            retry.disabled=true;const request=new AbortController();const deadline=setTimeout(()=>request.abort(),15000);
            try {const res=await fetch('/extensions/operational-health/api/projections/'+encodeURIComponent(projection.id)+'/retry?workspace='+encodeURIComponent(workspace),{method:'POST',credentials:'same-origin',redirect:'error',headers:{'Content-Type':'application/json'},body:'{}',signal:request.signal});if(!res.ok)throw new Error('Retry was not accepted. Check operational write access and refresh.');await loadEvidence(id,container);}catch(e){container.append(element('p',e.name==='AbortError'?'Retry outcome is uncertain. Refresh before trying again.':e.message,'unhealthy'));retry.disabled=false;}finally{clearTimeout(deadline);}
          });container.append(retry);
        }
      }
      for(const [label,cursor,kind]of [['More signal history',data.evidence.signals.next,'signals'],['More delivery records',data.evidence.projections.next,'projections']])if(cursor){const button=element('button',label);button.type='button';button.addEventListener('click',()=>loadEvidence(id,container,kind==='signals'?cursor:signalsAfter,kind==='projections'?cursor:projectionsAfter));container.append(button);}
    }catch(e){container.replaceChildren(element('p',e.name==='AbortError'?'Evidence timed out. Close and reopen to retry.':e.message,'unhealthy'));}finally{clearTimeout(timer);}
  }
  async function load(append = false) {
    if (controller) controller.abort(); controller = new AbortController();
    const request = ++generation; const activeController=controller; const deadline = setTimeout(() => activeController.abort(), 15000);
    refresh.disabled = true; more.disabled = true; error.hidden = true;
    try {
      const query = new URLSearchParams({limit:'20'}); if (select.value) query.set('environment',select.value); if (append && next) query.set('after',next);
      const [state,page] = await Promise.all([read('status',controller.signal),read('incidents?'+query.toString(),controller.signal)]);
      if (request !== generation) return;
      for (const env of [...new Set(state.targets.map(t => t.environment))].sort()) if (![...select.options].some(o => o.value === env)) {const option=element('option',env);option.value=env;select.append(option);}
      renderTargets(state.targets); renderIncidents(page.incidents,append); document.getElementById('checked').textContent='Checked '+new Date(state.checkedAt).toLocaleTimeString();
    } catch (failure) {
      if (request !== generation) return;
      error.textContent = failure.name === 'AbortError' ? 'The evidence request timed out. Retry to refresh it.' : failure.message; error.hidden=false;
      targets.replaceChildren(element('p','Current service health is unknown until refresh succeeds.','unknown')); document.getElementById('checked').textContent='Refresh failed';
    } finally { clearTimeout(deadline); if (request === generation) {refresh.disabled=false;more.disabled=false;} }
  }
  refresh.addEventListener('click',()=>load()); select.addEventListener('change',()=>load()); more.addEventListener('click',()=>load(true)); setInterval(()=>{if (!refresh.disabled && document.visibilityState === 'visible') load();},60000); load();
})();
